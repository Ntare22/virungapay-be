package company

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
	"virungapay-be/internal/middleware"
)

const pairUSDUGX = "USD/UGX"

func (s *Service) ListBureaus(ctx context.Context, role string) (BusinessList, error) {
	if !staff(role) {
		return BusinessList{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	businesses, err := s.store.ListBureaus(ctx)
	if err != nil {
		return BusinessList{}, middleware.Internal("internal_error", "internal server error")
	}
	if businesses == nil {
		businesses = []Business{}
	}
	return BusinessList{Businesses: businesses}, nil
}

// AddBureau creates a pending forex bureau and emails the owner an invite.
func (s *Service) AddBureau(ctx context.Context, role string, name, country, owner, email, phone string) (Business, error) {
	if !staff(role) {
		return Business{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	name = strings.TrimSpace(name)
	country = strings.ToUpper(strings.TrimSpace(country))
	owner = strings.TrimSpace(owner)
	email = strings.TrimSpace(email)
	if name == "" || owner == "" || email == "" || len(country) != 2 {
		return Business{}, middleware.Invalid("invalid_request", "name, country, owner, and email are required")
	}
	user, raw, tokenHash, err := s.auth.CreateInvitedUser(ctx, owner, email, phone, auth.RoleForexBureauOwner)
	if err != nil {
		return Business{}, err
	}
	now := s.now()
	org := Organisation{
		ID: uuid.NewString(), Type: "forex_bureau", Name: name, Country: country, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
	member := Member{OrganisationID: org.ID, UserID: user.ID, Role: auth.RoleForexBureauOwner, CreatedAt: now}
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		return s.store.InsertCompany(ctx, tx, org, member)
	}); err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.store.SaveInvite(ctx, Invite{
		ID: uuid.NewString(), OrganisationID: org.ID, UserID: user.ID, Role: auth.RoleForexBureauOwner,
		TokenHash: tokenHash, ExpiresAt: now.Add(inviteTTL), CreatedAt: now,
	}); err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	link := s.public + "/invites/accept?token=" + raw
	if err := s.mail.Send(ctx, mail.Message{
		ToEmail: user.Email, ToName: user.FullName(),
		Subject: "Your forex bureau on VirungaPay",
		Text:    "Set a password to open " + name + ": " + link,
		HTML:    "<p>Set a password to open " + name + ": <a href=\"" + link + "\">" + link + "</a></p>",
	}); err != nil {
		return Business{}, middleware.Unavailable("mail_unavailable", "email could not be sent")
	}
	return Business{
		ID: org.ID, Name: org.Name, Country: org.Country, Status: "pending",
		OwnerName: user.FullName(), OwnerEmail: user.Email, Wallets: []Wallet{},
	}, nil
}

func (s *Service) ListBusinesses(ctx context.Context, role string) (BusinessList, error) {
	if !staff(role) {
		return BusinessList{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	businesses, err := s.store.ListBusinesses(ctx)
	if err != nil {
		return BusinessList{}, middleware.Internal("internal_error", "internal server error")
	}
	if businesses == nil {
		businesses = []Business{}
	}
	return BusinessList{Businesses: businesses}, nil
}

func (s *Service) ListPersonWallets(ctx context.Context, role string) ([]PersonWallet, error) {
	if !staff(role) {
		return nil, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	wallets, err := s.store.ListPersonWallets(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if wallets == nil {
		wallets = []PersonWallet{}
	}
	return wallets, nil
}

func (s *Service) ListTourists(ctx context.Context, role string) ([]TouristAccount, error) {
	if role != auth.RoleOperatorOwner {
		return nil, middleware.Forbidden("forbidden", "only a tour operator can attach an itinerary")
	}
	list, err := s.store.ListTourists(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []TouristAccount{}
	}
	return list, nil
}

func (s *Service) Approve(ctx context.Context, actorID, role, organisationID string) (Business, error) {
	if !staff(role) {
		return Business{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	current, err := s.store.Company(ctx, organisationID)
	if errors.Is(err, errNotFound) {
		return Business{}, middleware.NotFound("business_not_found", "business was not found")
	}
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	if current.Status != "approved" {
		now := s.now()
		if err := s.tx(ctx, func(tx *gorm.DB) error {
			if err := s.store.SetStatus(ctx, tx, organisationID, "approved", now); err != nil {
				return err
			}
			wallets, err := s.store.Wallets(ctx, tx, organisationID)
			if err != nil {
				return err
			}
			for _, wallet := range wallets {
				if wallet.Currency == "UGX" {
					return nil
				}
			}
			kind := "operator"
			if current.Type == "forex_bureau" {
				kind = "bureau"
			}
			return s.store.InsertWallet(ctx, tx, Wallet{
				ID: uuid.NewString(), Kind: kind, Currency: "UGX", Default: true, Organisation: organisationID, CreatedBy: actorID,
			}, now)
		}); err != nil {
			return Business{}, middleware.Internal("internal_error", "internal server error")
		}
	}
	return s.business(ctx, organisationID)
}

func (s *Service) Disable(ctx context.Context, role, organisationID string) (Business, error) {
	if !staff(role) {
		return Business{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	current, err := s.store.Company(ctx, organisationID)
	if errors.Is(err, errNotFound) {
		return Business{}, middleware.NotFound("business_not_found", "business was not found")
	}
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	if current.Status != "approved" {
		return Business{}, middleware.Unprocessable("not_approved", "only an approved business can be disabled")
	}
	if err := s.store.SetStatus(ctx, s.store.DB(), organisationID, "disabled", s.now()); err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.business(ctx, organisationID)
}

func (s *Service) OpenWallet(ctx context.Context, actorID, role, organisationID, currency string) (Business, error) {
	if !staff(role) {
		return Business{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	if strings.ToUpper(strings.TrimSpace(currency)) != "USD" {
		return Business{}, middleware.Invalid("invalid_request", "only a USD wallet can be opened here")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	current, err := s.store.Company(ctx, organisationID)
	if errors.Is(err, errNotFound) {
		return Business{}, middleware.NotFound("business_not_found", "business was not found")
	}
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	if current.Status != "approved" {
		return Business{}, middleware.Unprocessable("business_not_approved", "approve the business before opening a wallet")
	}
	wallets, err := s.store.Wallets(ctx, s.store.DB(), organisationID)
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	for _, wallet := range wallets {
		if wallet.Currency == currency {
			return Business{}, middleware.Conflict("wallet_exists", "this business already has a "+currency+" wallet")
		}
	}
	kind := "operator"
	if current.Type == "forex_bureau" {
		kind = "bureau"
	}
	now := s.now()
	if err := s.store.InsertWallet(ctx, s.store.DB(), Wallet{
		ID: uuid.NewString(), Kind: kind, Currency: currency, Organisation: organisationID, CreatedBy: actorID,
	}, now); err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.business(ctx, organisationID)
}

func (s *Service) SetDefaultWallet(ctx context.Context, role, organisationID, walletID string) (Business, error) {
	if !staff(role) {
		return Business{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	wallets, err := s.store.Wallets(ctx, s.store.DB(), organisationID)
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	found := false
	for _, wallet := range wallets {
		if wallet.ID == walletID {
			found = true
		}
	}
	if !found {
		return Business{}, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	now := s.now()
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		if err := s.store.ClearDefaults(ctx, tx, organisationID, now); err != nil {
			return err
		}
		return s.store.MarkDefault(ctx, tx, walletID, now)
	}); err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.business(ctx, organisationID)
}

func (s *Service) TodayTariffs(ctx context.Context, role string) ([]Tariff, error) {
	switch role {
	case auth.RoleOperatorOwner, auth.RoleGuide, auth.RoleDriver:
	default:
		if !staff(role) {
			return nil, middleware.Forbidden("forbidden", "you cannot use this route")
		}
	}
	tariffs, err := s.store.Tariffs(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if tariffs == nil {
		tariffs = []Tariff{}
	}
	return orderTariffs(tariffs), nil
}

func (s *Service) SetTariff(ctx context.Context, actorID, role, pair, integrator string, markupBps int) (Tariff, error) {
	if !staff(role) {
		return Tariff{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	if pair != pairUSDUGX {
		return Tariff{}, middleware.Invalid("invalid_request", "choose USD/UGX")
	}
	if markupBps < 0 {
		return Tariff{}, middleware.Invalid("invalid_request", "markup cannot be negative")
	}
	customer, err := customerRate(integrator, markupBps)
	if err != nil {
		return Tariff{}, middleware.Invalid("invalid_request", "enter today's quotation as a positive number")
	}
	now := s.now()
	tariff := Tariff{
		ID: uuid.NewString(), Pair: pair, QuoteOn: s.quoteDate(),
		IntegratorRate: integrator, MarkupBps: markupBps, CustomerRate: customer,
	}
	if err := s.store.SaveTariff(ctx, tariff, actorID, now); err != nil {
		return Tariff{}, middleware.Internal("internal_error", "internal server error")
	}
	return tariff, nil
}

func (s *Service) withWallets(ctx context.Context, company Company) (Company, error) {
	wallets, err := s.store.Wallets(ctx, s.store.DB(), company.ID)
	if err != nil {
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	if wallets == nil {
		wallets = []Wallet{}
	}
	company.Wallets = wallets
	return company, nil
}

func (s *Service) business(ctx context.Context, organisationID string) (Business, error) {
	listed, err := s.store.ListBusinesses(ctx)
	if err != nil {
		return Business{}, middleware.Internal("internal_error", "internal server error")
	}
	for _, business := range listed {
		if business.ID == organisationID {
			if business.Wallets == nil {
				business.Wallets = []Wallet{}
			}
			return business, nil
		}
	}
	return Business{}, middleware.NotFound("business_not_found", "business was not found")
}

func (s *Service) quoteDate() string {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		loc = time.FixedZone("EAT", 3*60*60)
	}
	return s.now().In(loc).Format("2006-01-02")
}

func staff(role string) bool {
	return role == auth.RoleVAOps || role == auth.RoleVAFinance || role == auth.RoleVASuperadmin
}

func orderTariffs(tariffs []Tariff) []Tariff {
	ordered := make([]Tariff, 0, 1)
	for _, pair := range []string{pairUSDUGX} {
		for _, tariff := range tariffs {
			if tariff.Pair == pair {
				ordered = append(ordered, tariff)
			}
		}
	}
	return ordered
}

func customerRate(integrator string, markupBps int) (string, error) {
	rate, ok := new(big.Rat).SetString(strings.TrimSpace(integrator))
	if !ok || rate.Sign() <= 0 {
		return "", errors.New("invalid rate")
	}
	factor := new(big.Rat).SetInt64(int64(10000 + markupBps))
	rate.Mul(rate, factor)
	rate.Quo(rate, big.NewRat(10000, 1))
	return rate.FloatString(8), nil
}
