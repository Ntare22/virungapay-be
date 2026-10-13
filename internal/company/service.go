package company

import (
	"context"
	"errors"
	"strings"
	"time"
	"virungapay-be/internal/middleware"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
)

const inviteTTL = 7 * 24 * time.Hour

// Tx runs a function inside one database transaction.
type Tx func(ctx context.Context, fn func(tx *gorm.DB) error) error

// Registrar creates the operator owner account. The auth module owns passwords and verification email.
type Registrar interface {
	RegisterOperator(ctx context.Context, in auth.Signup) (auth.User, error)
	RegisterForexBureau(ctx context.Context, in auth.Signup) (auth.User, error)
	CreateInvitedUser(ctx context.Context, name, email, phone, role string) (auth.User, string, string, error)
}

// Store is the company persistence used by the service.
type Store interface {
	Member(ctx context.Context, db *gorm.DB, userID string) (Member, error)
	Company(ctx context.Context, organisationID string) (Company, error)
	InsertCompany(ctx context.Context, tx *gorm.DB, org Organisation, member Member) error
	SaveMember(ctx context.Context, member Member) error
	SaveInvite(ctx context.Context, invite Invite) error
	Wallets(ctx context.Context, db *gorm.DB, organisationID string) ([]Wallet, error)
	InsertWallet(ctx context.Context, db *gorm.DB, wallet Wallet, at time.Time) error
	SetStatus(ctx context.Context, db *gorm.DB, organisationID, status string, at time.Time) error
	ClearDefaults(ctx context.Context, db *gorm.DB, organisationID string, at time.Time) error
	MarkDefault(ctx context.Context, db *gorm.DB, walletID string, at time.Time) error
	ListBusinesses(ctx context.Context) ([]Business, error)
	ListBureaus(ctx context.Context) ([]Business, error)
	Tariffs(ctx context.Context) ([]Tariff, error)
	SaveTariff(ctx context.Context, tariff Tariff, setBy string, at time.Time) error
	ListFeeSchedules(ctx context.Context) ([]FeeSchedule, error)
	FeeSchedule(ctx context.Context, db *gorm.DB, kind, currency string) (FeeSchedule, error)
	SaveFeeSchedule(ctx context.Context, schedule FeeSchedule, actorID string, at time.Time) error
	ListPlatformWallets(ctx context.Context) ([]Wallet, error)
	ListPersonWallets(ctx context.Context) ([]PersonWallet, error)
	ListTourists(ctx context.Context) ([]TouristAccount, error)
	PlatformWallet(ctx context.Context, db *gorm.DB, kind, currency string) (Wallet, error)
	PlatformWalletByID(ctx context.Context, walletID string) (Wallet, error)
	ListMovements(ctx context.Context, walletID string) ([]Movement, error)
	PersonalWallet(ctx context.Context, db *gorm.DB, userID, currency string) (Wallet, error)
	PersonalWallets(ctx context.Context, userID string) ([]Wallet, error)
	AdjustBalance(ctx context.Context, db *gorm.DB, walletID, delta string, at time.Time) error
	InsertMovement(ctx context.Context, db *gorm.DB, movement Movement) error
	InsertTransaction(ctx context.Context, db *gorm.DB, txn TransactionRecord) error
	ListTransactions(ctx context.Context) ([]Transaction, error)
	ListWalletActivity(ctx context.Context, organisationID string) ([]WalletActivity, error)
	ListPersonalActivity(ctx context.Context, userID string) ([]WalletActivity, error)
	UserRole(ctx context.Context, userID string) (string, error)
	ListCrew(ctx context.Context, organisationID string) ([]CrewMember, error)
	FindByUsername(ctx context.Context, username string) (Wallet, string, error)
	WalletByID(ctx context.Context, walletID string) (Wallet, error)
	RenameWallet(ctx context.Context, walletID, name, username string, at time.Time) error
	DB() *gorm.DB
}

// Service creates one company per operator owner and invites guides and drivers.
type Service struct {
	store    Store
	auth     Registrar
	mail     mail.Sender
	tx       Tx
	public   string
	now      func() time.Time
	fallback *gorm.DB
}

// NewService builds the company service. public is the app origin used in invite links.
func NewService(store Store, registrar Registrar, sender mail.Sender, run Tx, publicURL string) *Service {
	return &Service{
		store:  store,
		auth:   registrar,
		mail:   sender,
		tx:     run,
		public: strings.TrimRight(publicURL, "/"),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// RegisterOperator creates an unverified operator owner. It does not create a company.
func (s *Service) RegisterOperator(ctx context.Context, in auth.Signup) (auth.User, error) {
	return s.auth.RegisterOperator(ctx, in)
}

// RegisterForexBureau creates an unverified forex bureau owner. It does not create the bureau.
func (s *Service) RegisterForexBureau(ctx context.Context, in auth.Signup) (auth.User, error) {
	return s.auth.RegisterForexBureau(ctx, in)
}

// Create writes one operator organisation and the owner membership.
func (s *Service) Create(ctx context.Context, userID, role string, in CreateInput) (Company, error) {
	if role != auth.RoleOperatorOwner && role != auth.RoleForexBureauOwner {
		return Company{}, middleware.Forbidden("forbidden", "only a business owner can create a company")
	}
	orgType := "operator"
	if role == auth.RoleForexBureauOwner {
		orgType = "forex_bureau"
	}
	name := strings.TrimSpace(in.Name)
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if name == "" || len(country) != 2 {
		return Company{}, middleware.Invalid("invalid_request", "name and an ISO country are required")
	}
	association, err := associationCode(in.Association)
	if err != nil {
		return Company{}, err
	}
	if _, err := s.store.Member(ctx, s.store.DB(), userID); err == nil {
		return Company{}, middleware.Conflict("company_exists", "this account already has a company")
	} else if !errors.Is(err, errNotFound) {
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}

	now := s.now()
	org := Organisation{
		ID:                  uuid.NewString(),
		Type:                orgType,
		Name:                name,
		Country:             country,
		AssociationMemberOf: association,
		Status:              "pending",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	member := Member{OrganisationID: org.ID, UserID: userID, Role: role, CreatedAt: now}
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		if _, err := s.store.Member(ctx, tx, userID); err == nil {
			return middleware.Conflict("company_exists", "this account already has a company")
		} else if !errors.Is(err, errNotFound) {
			return err
		}
		return s.store.InsertCompany(ctx, tx, org, member)
	}); err != nil {
		var app *middleware.AppError
		if errors.As(err, &app) {
			return Company{}, app
		}
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	return Company{ID: org.ID, Name: org.Name, Country: org.Country, AssociationMemberOf: association, Status: "pending", Type: orgType, Wallets: []Wallet{}}, nil
}

// Mine returns the caller's company.
func (s *Service) Mine(ctx context.Context, userID, role string) (Company, error) {
	if role != auth.RoleOperatorOwner && role != auth.RoleForexBureauOwner {
		return Company{}, middleware.Forbidden("forbidden", "only a business owner can view this company")
	}
	member, err := s.store.Member(ctx, s.store.DB(), userID)
	if errors.Is(err, errNotFound) {
		return Company{}, middleware.NotFound("company_not_found", "create a company first")
	}
	if err != nil {
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	company, err := s.store.Company(ctx, member.OrganisationID)
	if err != nil {
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.withWallets(ctx, company)
}

// Invite creates an invited guide or driver on this company and emails the link.
func (s *Service) Invite(ctx context.Context, userID, role string, in InviteInput) (InviteResponse, error) {
	if role != auth.RoleOperatorOwner {
		return InviteResponse{}, middleware.Forbidden("forbidden", "only an operator owner can invite crew")
	}
	member, err := s.store.Member(ctx, s.store.DB(), userID)
	if errors.Is(err, errNotFound) {
		return InviteResponse{}, middleware.Unprocessable("company_required", "create a company before inviting crew")
	}
	if err != nil {
		return InviteResponse{}, middleware.Internal("internal_error", "internal server error")
	}
	user, raw, tokenHash, err := s.auth.CreateInvitedUser(ctx, in.Name, in.Email, in.Phone, in.Role)
	if err != nil {
		return InviteResponse{}, err
	}
	now := s.now()
	if err := s.store.SaveMember(ctx, Member{
		OrganisationID: member.OrganisationID, UserID: user.ID, Role: user.Role, CreatedAt: now,
	}); err != nil {
		return InviteResponse{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.openCrewWallet(ctx, user.ID, in.Name); err != nil {
		return InviteResponse{}, err
	}
	invite := Invite{
		ID:             uuid.NewString(),
		OrganisationID: member.OrganisationID,
		UserID:         user.ID,
		Role:           user.Role,
		TokenHash:      tokenHash,
		ExpiresAt:      now.Add(inviteTTL),
		CreatedAt:      now,
	}
	if err := s.store.SaveInvite(ctx, invite); err != nil {
		return InviteResponse{}, middleware.Internal("internal_error", "internal server error")
	}
	link := s.public + "/invites/accept?token=" + raw
	if err := s.mail.Send(ctx, mail.Message{
		ToEmail: user.Email,
		ToName:  user.FullName(),
		Subject: "You are invited to VirungaPay",
		Text:    "Accept your invite: " + link,
		HTML:    "<p>Accept your invite: <a href=\"" + link + "\">" + link + "</a></p>",
	}); err != nil {
		return InviteResponse{}, middleware.Unavailable("mail_unavailable", "email could not be sent")
	}
	return InviteResponse{UserID: user.ID, Email: user.Email, Role: user.Role, Status: auth.StatusInvited}, nil
}

// ListCrew returns the operator's guides and drivers. Each one has a UGX wallet.
func (s *Service) ListCrew(ctx context.Context, userID, role string) ([]CrewMember, error) {
	if role != auth.RoleOperatorOwner {
		return nil, middleware.Forbidden("forbidden", "only an operator owner can view guides and drivers")
	}
	member, err := s.store.Member(ctx, s.store.DB(), userID)
	if errors.Is(err, errNotFound) {
		return nil, middleware.Unprocessable("company_required", "create a company before adding guides and drivers")
	}
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	list, err := s.store.ListCrew(ctx, member.OrganisationID)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []CrewMember{}
	}
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	for i := range list {
		if list[i].WalletCurrency == currency && list[i].WalletBalance != "" {
			continue
		}
		if err := s.openCrewWallet(ctx, list[i].UserID, list[i].Name); err != nil {
			return nil, err
		}
		list[i].WalletCurrency = currency
		if list[i].WalletBalance == "" {
			list[i].WalletBalance = "0.00"
		}
	}
	return list, nil
}

// LookupWallet confirms a username belongs to a wallet a sender can pay.
func (s *Service) LookupWallet(ctx context.Context, role, username string) (WalletLookup, error) {
	switch role {
	case auth.RoleOperatorOwner, auth.RoleGuide, auth.RoleDriver, auth.RoleTourist:
	default:
		return WalletLookup{}, middleware.Forbidden("forbidden", "you cannot look up a wallet")
	}
	username = normalizeUsername(username)
	if username == "" {
		return WalletLookup{}, middleware.Invalid("invalid_request", "enter a wallet username")
	}
	wallet, name, err := s.store.FindByUsername(ctx, username)
	if errors.Is(err, errNotFound) || (err == nil && (wallet.Kind == "ova" || strings.HasPrefix(wallet.Kind, "fee_"))) {
		return WalletLookup{}, middleware.NotFound("wallet_not_found", "no wallet uses that username")
	}
	if err != nil {
		return WalletLookup{}, middleware.Internal("internal_error", "internal server error")
	}
	if name == "" {
		name = wallet.Name
	}
	return WalletLookup{Username: wallet.Username, Name: name, Currency: wallet.Currency, Kind: wallet.Kind}, nil
}

func (s *Service) openCrewWallet(ctx context.Context, userID, name string) error {
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return middleware.Internal("internal_error", "internal server error")
	}
	_, err := s.store.PersonalWallet(ctx, s.store.DB(), userID, currency)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errNotFound) {
		return middleware.Internal("internal_error", "internal server error")
	}
	created := Wallet{
		ID: uuid.NewString(), Kind: "personal", Name: strings.TrimSpace(name), Currency: currency, Balance: "0.00", Default: true, UserID: userID,
	}
	if err := s.store.InsertWallet(ctx, s.store.DB(), created, s.now()); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}

func (r *Repository) DB() *gorm.DB { return r.db }

func associationCode(raw string) (*string, error) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		return nil, nil
	}
	if code != "AUTO" && code != "UHOA" {
		return nil, middleware.Invalid("invalid_association", "association must be AUTO or UHOA")
	}
	return &code, nil
}
