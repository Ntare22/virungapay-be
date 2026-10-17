package company

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"virungapay-be/internal/auth"
)

// Repository writes organisations, memberships, and invites.
type Repository struct {
	db *gorm.DB
}

// NewRepository uses the application GORM handle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) OrganisationID(ctx context.Context, userID string) (string, error) {
	var row memberRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.OrganisationID, nil
}

func (r *Repository) FindByTokenHash(ctx context.Context, hash string) (auth.PendingInvite, error) {
	var row inviteRow
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.PendingInvite{}, errNotFound
	}
	if err != nil {
		return auth.PendingInvite{}, err
	}
	return auth.PendingInvite{
		ID: row.ID, OrganisationID: row.OrganisationID, UserID: row.UserID,
		Role: row.Role, ExpiresAt: row.ExpiresAt, AcceptedAt: row.AcceptedAt,
	}, nil
}

func (r *Repository) MarkAccepted(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&inviteRow{}).Where("id = ?", id).Update("accepted_at", at).Error
}

func (r *Repository) Member(ctx context.Context, db *gorm.DB, userID string) (Member, error) {
	var row memberRow
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Member{}, errNotFound
	}
	return Member{OrganisationID: row.OrganisationID, UserID: row.UserID, Role: row.MemberRole, CreatedAt: row.CreatedAt}, err
}

func (r *Repository) Company(ctx context.Context, organisationID string) (Company, error) {
	var row Organisation
	err := r.db.WithContext(ctx).Where("id = ?", organisationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Company{}, errNotFound
	}
	if err != nil {
		return Company{}, err
	}
	return Company{ID: row.ID, Name: row.Name, Country: row.Country, AssociationMemberOf: row.AssociationMemberOf, Status: row.Status, Type: row.Type, Wallets: []Wallet{}}, nil
}

// InsertCompany writes the organisation and the owner membership.
func (r *Repository) InsertCompany(ctx context.Context, tx *gorm.DB, org Organisation, member Member) error {
	if err := tx.WithContext(ctx).Create(&org).Error; err != nil {
		return err
	}
	row := memberRow{
		OrganisationID: member.OrganisationID,
		UserID:         member.UserID,
		MemberRole:     member.Role,
		CreatedAt:      member.CreatedAt,
	}
	return tx.WithContext(ctx).Create(&row).Error
}

func (r *Repository) SaveMember(ctx context.Context, member Member) error {
	row := memberRow{
		OrganisationID: member.OrganisationID,
		UserID:         member.UserID,
		MemberRole:     member.Role,
		CreatedAt:      member.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) SaveInvite(ctx context.Context, invite Invite) error {
	row := inviteRow{
		ID: invite.ID, OrganisationID: invite.OrganisationID, UserID: invite.UserID,
		Role: invite.Role, TokenHash: invite.TokenHash, ExpiresAt: invite.ExpiresAt, CreatedAt: invite.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

type memberRow struct {
	OrganisationID string    `gorm:"type:uuid;primaryKey"`
	UserID         string    `gorm:"type:uuid;primaryKey"`
	MemberRole     string    `gorm:"column:member_role;not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (memberRow) TableName() string { return "organisation_members" }

type inviteRow struct {
	ID             string `gorm:"type:uuid;primaryKey"`
	OrganisationID string `gorm:"type:uuid;not null"`
	UserID         string `gorm:"type:uuid;not null"`
	Role           string `gorm:"not null"`
	TokenHash      string `gorm:"not null"`
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	CreatedAt      time.Time
}

func (inviteRow) TableName() string { return "company_invites" }

type walletRow struct {
	ID             string  `gorm:"type:uuid;primaryKey"`
	OrganisationID *string `gorm:"type:uuid"`
	UserID         *string `gorm:"type:uuid"`
	Kind           string  `gorm:"not null"`
	Name           string  `gorm:"not null"`
	Username       string  `gorm:"not null"`
	Currency       string  `gorm:"not null"`
	Balance        string  `gorm:"type:numeric(20,2);not null"`
	IsDefault      bool    `gorm:"column:is_default;not null"`
	CreatedBy      *string `gorm:"type:uuid"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (walletRow) TableName() string { return "wallets" }

func (r *Repository) Wallets(ctx context.Context, db *gorm.DB, organisationID string) ([]Wallet, error) {
	var rows []walletRow
	err := db.WithContext(ctx).Where("organisation_id = ?", organisationID).Order("currency").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	wallets := make([]Wallet, 0, len(rows))
	for _, row := range rows {
		wallets = append(wallets, walletFromRow(row))
	}
	return wallets, nil
}

func walletFromRow(row walletRow) Wallet {
	wallet := Wallet{
		ID: row.ID, Kind: row.Kind, Name: row.Name, Username: row.Username, Currency: row.Currency, Balance: row.Balance, Default: row.IsDefault,
	}
	if wallet.Balance == "" {
		wallet.Balance = "0.00"
	}
	if row.OrganisationID != nil {
		wallet.Organisation = *row.OrganisationID
	}
	if row.UserID != nil {
		wallet.UserID = *row.UserID
	}
	if row.CreatedBy != nil {
		wallet.CreatedBy = *row.CreatedBy
	}
	return wallet
}

func (r *Repository) InsertWallet(ctx context.Context, db *gorm.DB, wallet Wallet, at time.Time) error {
	kind := wallet.Kind
	if kind == "" {
		kind = "operator"
	}
	balance := wallet.Balance
	if balance == "" {
		balance = "0.00"
	}
	username := normalizeUsername(wallet.Username)
	if username == "" {
		var err error
		username, err = r.nextUsername(ctx, db, r.usernameBase(ctx, db, wallet), wallet.Currency)
		if err != nil {
			return err
		}
	}
	row := walletRow{
		ID: wallet.ID, Kind: kind, Name: wallet.Name, Username: username, Currency: wallet.Currency, Balance: balance,
		IsDefault: wallet.Default, CreatedAt: at, UpdatedAt: at,
	}
	if wallet.Organisation != "" {
		row.OrganisationID = &wallet.Organisation
	}
	if wallet.UserID != "" {
		row.UserID = &wallet.UserID
	}
	if wallet.CreatedBy != "" {
		row.CreatedBy = &wallet.CreatedBy
	}
	return db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) WalletByID(ctx context.Context, walletID string) (Wallet, error) {
	var row walletRow
	err := r.db.WithContext(ctx).Where("id = ?", walletID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Wallet{}, errNotFound
	}
	if err != nil {
		return Wallet{}, err
	}
	return walletFromRow(row), nil
}

func (r *Repository) RenameWallet(ctx context.Context, walletID, name, username string, at time.Time) error {
	result := r.db.WithContext(ctx).Model(&walletRow{}).Where("id = ?", walletID).
		Updates(map[string]any{"name": name, "username": username, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) SetStatus(ctx context.Context, db *gorm.DB, organisationID, status string, at time.Time) error {
	return db.WithContext(ctx).Model(&Organisation{}).Where("id = ?", organisationID).
		Updates(map[string]any{"status": status, "updated_at": at}).Error
}

func (r *Repository) ClearDefaults(ctx context.Context, db *gorm.DB, organisationID string, at time.Time) error {
	return db.WithContext(ctx).Model(&walletRow{}).Where("organisation_id = ?", organisationID).
		Updates(map[string]any{"is_default": false, "updated_at": at}).Error
}

func (r *Repository) MarkDefault(ctx context.Context, db *gorm.DB, walletID string, at time.Time) error {
	return db.WithContext(ctx).Model(&walletRow{}).Where("id = ?", walletID).
		Updates(map[string]any{"is_default": true, "updated_at": at}).Error
}

type businessJoin struct {
	ID                  string
	Name                string
	Country             string
	AssociationMemberOf *string
	Status              string
	FirstName           string
	LastName            string
	Email               string
}

func (r *Repository) ListBusinesses(ctx context.Context) ([]Business, error) {
	return r.listOrganisations(ctx, "operator", auth.RoleOperatorOwner)
}

func (r *Repository) ListBureaus(ctx context.Context) ([]Business, error) {
	return r.listOrganisations(ctx, "forex_bureau", auth.RoleForexBureauOwner)
}

func (r *Repository) listOrganisations(ctx context.Context, orgType, ownerRole string) ([]Business, error) {
	var rows []businessJoin
	err := r.db.WithContext(ctx).Table("organisations").
		Select("organisations.id, organisations.name, organisations.country, organisations.association_member_of, organisations.status, users.first_name, users.last_name, users.email").
		Joins("JOIN organisation_members ON organisation_members.organisation_id = organisations.id AND organisation_members.member_role = ?", ownerRole).
		Joins("JOIN users ON users.id = organisation_members.user_id").
		Where("organisations.type = ?", orgType).
		Order("organisations.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	businesses := make([]Business, 0, len(rows))
	for _, row := range rows {
		wallets, err := r.Wallets(ctx, r.db, row.ID)
		if err != nil {
			return nil, err
		}
		name := stringsTrim(row.FirstName, row.LastName)
		businesses = append(businesses, Business{
			ID: row.ID, Name: row.Name, Country: row.Country, AssociationMemberOf: row.AssociationMemberOf,
			Status: row.Status, OwnerName: name, OwnerEmail: row.Email, Wallets: wallets,
		})
	}
	return businesses, nil
}

func stringsTrim(first, last string) string {
	if last == "" {
		return first
	}
	return first + " " + last
}

type tariffRow struct {
	ID             string `gorm:"type:uuid;primaryKey"`
	Pair           string
	QuoteOn        time.Time `gorm:"column:quote_on;type:date"`
	IntegratorRate string    `gorm:"column:integrator_rate"`
	MarkupBps      int       `gorm:"column:markup_bps"`
	CustomerRate   string    `gorm:"column:customer_rate"`
	SetBy          string    `gorm:"column:set_by"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (tariffRow) TableName() string { return "fx_rates" }

func (r *Repository) Tariffs(ctx context.Context) ([]Tariff, error) {
	var rows []tariffRow
	err := r.db.WithContext(ctx).Raw(`
SELECT DISTINCT ON (pair) id, pair, quote_on, integrator_rate::text AS integrator_rate, markup_bps, customer_rate::text AS customer_rate, set_by, created_at, updated_at
FROM fx_rates
WHERE pair IN ('USD/UGX')
ORDER BY pair, quote_on DESC
`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	tariffs := make([]Tariff, 0, len(rows))
	for _, row := range rows {
		tariffs = append(tariffs, Tariff{
			Pair: row.Pair, QuoteOn: row.QuoteOn.Format("2006-01-02"), IntegratorRate: row.IntegratorRate,
			MarkupBps: row.MarkupBps, CustomerRate: row.CustomerRate,
		})
	}
	return tariffs, nil
}

func (r *Repository) SaveTariff(ctx context.Context, tariff Tariff, setBy string, at time.Time) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO fx_rates (id, pair, quote_on, integrator_rate, markup_bps, customer_rate, set_by, created_at, updated_at)
VALUES (?, ?, ?::date, ?::numeric, ?, ?::numeric, ?, ?, ?)
ON CONFLICT (pair, quote_on) DO UPDATE SET
    integrator_rate = EXCLUDED.integrator_rate,
    markup_bps = EXCLUDED.markup_bps,
    customer_rate = EXCLUDED.customer_rate,
    set_by = EXCLUDED.set_by,
    updated_at = EXCLUDED.updated_at
`, tariff.ID, tariff.Pair, tariff.QuoteOn, tariff.IntegratorRate, tariff.MarkupBps, tariff.CustomerRate, setBy, at, at).Error
}

var errNotFound = errors.New("not found")

func (r *Repository) PlatformWallet(ctx context.Context, db *gorm.DB, kind, currency string) (Wallet, error) {
	var row walletRow
	err := db.WithContext(ctx).Where("kind = ? AND currency = ? AND organisation_id IS NULL AND user_id IS NULL", kind, currency).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Wallet{}, errNotFound
	}
	if err != nil {
		return Wallet{}, err
	}
	return walletFromRow(row), nil
}

func (r *Repository) PlatformWalletByID(ctx context.Context, walletID string) (Wallet, error) {
	var row walletRow
	err := r.db.WithContext(ctx).Where("id = ? AND organisation_id IS NULL AND user_id IS NULL", walletID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Wallet{}, errNotFound
	}
	if err != nil {
		return Wallet{}, err
	}
	return walletFromRow(row), nil
}

func (r *Repository) ListMovements(ctx context.Context, walletID string) ([]Movement, error) {
	var rows []movementRow
	err := r.db.WithContext(ctx).Where("wallet_id = ?", walletID).Order("created_at ASC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	movements := make([]Movement, 0, len(rows))
	for _, row := range rows {
		movements = append(movements, Movement{
			ID: row.ID, GroupID: row.GroupID, Kind: row.Kind, WalletID: row.WalletID,
			Direction: row.Direction, Amount: row.Amount, Currency: row.Currency, CreatedAt: row.CreatedAt,
		})
	}
	return movements, nil
}

func (r *Repository) ListPlatformWallets(ctx context.Context) ([]Wallet, error) {
	var rows []walletRow
	err := r.db.WithContext(ctx).Where("organisation_id IS NULL AND user_id IS NULL").Order("kind, currency").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	wallets := make([]Wallet, 0, len(rows))
	for _, row := range rows {
		wallets = append(wallets, walletFromRow(row))
	}
	return wallets, nil
}

func (r *Repository) ListPersonWallets(ctx context.Context) ([]PersonWallet, error) {
	var rows []PersonWallet
	err := r.db.WithContext(ctx).Raw(`
SELECT wallets.id, wallets.kind, wallets.name, COALESCE(wallets.username, '') AS username, wallets.currency, wallets.balance::text AS balance,
       trim(concat_ws(' ', users.first_name, users.last_name)) AS holder_name, users.email AS holder_email, users.role AS holder_role, users.status AS holder_status
FROM wallets
JOIN users ON users.id = wallets.user_id
WHERE wallets.kind IN ('virungapass', 'personal')
ORDER BY users.role, users.last_name, users.first_name, wallets.currency
`).Scan(&rows).Error
	return rows, err
}

func (r *Repository) ListTourists(ctx context.Context) ([]TouristAccount, error) {
	var rows []TouristAccount
	err := r.db.WithContext(ctx).Raw(`
SELECT users.id, trim(concat_ws(' ', users.first_name, users.last_name)) AS name, users.email,
       COALESCE(wallets.username, '') AS username
FROM users
LEFT JOIN wallets ON wallets.user_id = users.id AND wallets.kind = 'virungapass'
WHERE users.role = 'tourist'
ORDER BY users.last_name, users.first_name
`).Scan(&rows).Error
	return rows, err
}

func (r *Repository) PersonalWallet(ctx context.Context, db *gorm.DB, userID, currency string) (Wallet, error) {
	var row walletRow
	err := db.WithContext(ctx).Where("user_id = ? AND currency = ?", userID, currency).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Wallet{}, errNotFound
	}
	if err != nil {
		return Wallet{}, err
	}
	return walletFromRow(row), nil
}

func (r *Repository) PersonalWallets(ctx context.Context, userID string) ([]Wallet, error) {
	var rows []walletRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("currency").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	wallets := make([]Wallet, 0, len(rows))
	for _, row := range rows {
		wallets = append(wallets, walletFromRow(row))
	}
	return wallets, nil
}

func (r *Repository) AdjustBalance(ctx context.Context, db *gorm.DB, walletID, delta string, at time.Time) error {
	result := db.WithContext(ctx).Exec(`
UPDATE wallets
SET balance = balance + ?::numeric, updated_at = ?
WHERE id = ? AND balance + ?::numeric >= 0
`, delta, at, walletID, delta)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

type movementRow struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	GroupID   string `gorm:"type:uuid;not null"`
	Kind      string `gorm:"not null"`
	WalletID  string `gorm:"type:uuid;not null"`
	Direction string `gorm:"not null"`
	Amount    string `gorm:"type:numeric(20,2);not null"`
	Currency  string `gorm:"not null"`
	CreatedAt time.Time
}

func (movementRow) TableName() string { return "wallet_movements" }

func (r *Repository) InsertMovement(ctx context.Context, db *gorm.DB, movement Movement) error {
	row := movementRow{
		ID: movement.ID, GroupID: movement.GroupID, Kind: movement.Kind, WalletID: movement.WalletID,
		Direction: movement.Direction, Amount: movement.Amount, Currency: movement.Currency, CreatedAt: movement.CreatedAt,
	}
	return db.WithContext(ctx).Create(&row).Error
}

type transactionRow struct {
	ID                string `gorm:"type:uuid;primaryKey"`
	GroupID           string `gorm:"type:uuid"`
	Kind              string
	Currency          string
	Gross             string `gorm:"type:numeric(20,2)"`
	Fee               string `gorm:"type:numeric(20,2)"`
	Net               string `gorm:"type:numeric(20,2)"`
	OrganisationID    *string
	FromName          string
	ToName            string
	CounterpartyEmail string
	ItineraryID       *string
	InvoiceNumber     string
	QuoteCurrency     string
	QuoteAmount       *string `gorm:"type:numeric(20,2)"`
	CreatedAt         time.Time
}

func (transactionRow) TableName() string { return "transactions" }

func (r *Repository) InsertTransaction(ctx context.Context, db *gorm.DB, txn TransactionRecord) error {
	row := transactionRow{
		ID: txn.ID, GroupID: txn.GroupID, Kind: txn.Kind, Currency: txn.Currency,
		Gross: txn.Gross, Fee: txn.Fee, Net: txn.Net, FromName: txn.FromName, ToName: txn.ToName,
		CounterpartyEmail: txn.CounterpartyEmail, InvoiceNumber: txn.InvoiceNumber, QuoteCurrency: txn.QuoteCurrency, CreatedAt: txn.CreatedAt,
	}
	if txn.QuoteAmount != "" {
		amount := txn.QuoteAmount
		row.QuoteAmount = &amount
	}
	if txn.OrganisationID != "" {
		row.OrganisationID = &txn.OrganisationID
	}
	if txn.ItineraryID != "" {
		row.ItineraryID = &txn.ItineraryID
	}
	return db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListTransactions(ctx context.Context) ([]Transaction, error) {
	var rows []transactionRow
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]Transaction, 0, len(rows))
	for _, row := range rows {
		item := Transaction{
			ID: row.ID, Kind: row.Kind, Currency: row.Currency, Gross: row.Gross, Fee: row.Fee, Net: row.Net,
			FromName: row.FromName, ToName: row.ToName, InvoiceNumber: row.InvoiceNumber, QuoteCurrency: row.QuoteCurrency, CreatedAt: row.CreatedAt,
		}
		if row.QuoteAmount != nil {
			item.QuoteAmount = *row.QuoteAmount
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) ListWalletActivity(ctx context.Context, organisationID string) ([]WalletActivity, error) {
	var rows []struct {
		ID            string
		Kind          string
		Direction     string
		Amount        string
		Currency      string
		InvoiceNumber string
		FromName      string
		ToName        string
		CreatedAt     time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
SELECT movement.id, movement.kind, movement.direction, movement.amount::text, movement.currency,
       COALESCE(txn.invoice_number, '') AS invoice_number,
       COALESCE(txn.from_name, '') AS from_name,
       COALESCE(txn.to_name, '') AS to_name,
       movement.created_at
FROM wallet_movements AS movement
JOIN wallets AS wallet ON wallet.id = movement.wallet_id
LEFT JOIN transactions AS txn ON txn.group_id = movement.group_id
WHERE wallet.organisation_id = ?
ORDER BY movement.created_at DESC
`, organisationID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]WalletActivity, 0, len(rows))
	for _, row := range rows {
		party := row.FromName
		if row.Direction == "out" {
			party = row.ToName
		}
		list = append(list, WalletActivity{
			ID: row.ID, Kind: row.Kind, Direction: row.Direction, Amount: row.Amount, Currency: row.Currency,
			InvoiceNumber: row.InvoiceNumber, Counterparty: party, CreatedAt: row.CreatedAt,
		})
	}
	return list, nil
}

func (r *Repository) ListPersonalActivity(ctx context.Context, userID string) ([]WalletActivity, error) {
	var rows []struct {
		ID            string
		Kind          string
		Direction     string
		Amount        string
		Currency      string
		InvoiceNumber string
		FromName      string
		ToName        string
		CreatedAt     time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
SELECT movement.id, movement.kind, movement.direction, movement.amount::text, movement.currency,
       COALESCE(txn.invoice_number, '') AS invoice_number,
       COALESCE(txn.from_name, '') AS from_name,
       COALESCE(txn.to_name, '') AS to_name,
       movement.created_at
FROM wallet_movements AS movement
JOIN wallets AS wallet ON wallet.id = movement.wallet_id
LEFT JOIN transactions AS txn ON txn.group_id = movement.group_id
WHERE wallet.user_id = ?
ORDER BY movement.created_at DESC
`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]WalletActivity, 0, len(rows))
	for _, row := range rows {
		party := row.FromName
		if row.Direction == "out" {
			party = row.ToName
		}
		list = append(list, WalletActivity{
			ID: row.ID, Kind: row.Kind, Direction: row.Direction, Amount: row.Amount, Currency: row.Currency,
			InvoiceNumber: row.InvoiceNumber, Counterparty: party, CreatedAt: row.CreatedAt,
		})
	}
	return list, nil
}

type feeRow struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	Kind      string
	Currency  string
	Flat      string `gorm:"type:numeric(20,2)"`
	MarkupBps int    `gorm:"column:markup_bps"`
	UpdatedBy *string
	UpdatedAt time.Time
}

func (feeRow) TableName() string { return "transaction_fees" }

func (r *Repository) ListFeeSchedules(ctx context.Context) ([]FeeSchedule, error) {
	var rows []feeRow
	err := r.db.WithContext(ctx).Order("kind, currency").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	schedules := make([]FeeSchedule, 0, len(rows))
	for _, row := range rows {
		schedules = append(schedules, FeeSchedule{Kind: row.Kind, Currency: row.Currency, Flat: row.Flat, MarkupBps: row.MarkupBps})
	}
	return schedules, nil
}

func (r *Repository) FeeSchedule(ctx context.Context, db *gorm.DB, kind, currency string) (FeeSchedule, error) {
	var row feeRow
	err := db.WithContext(ctx).Where("kind = ? AND currency = ?", kind, currency).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FeeSchedule{}, errNotFound
	}
	if err != nil {
		return FeeSchedule{}, err
	}
	return FeeSchedule{Kind: row.Kind, Currency: row.Currency, Flat: row.Flat, MarkupBps: row.MarkupBps}, nil
}

func (r *Repository) SaveFeeSchedule(ctx context.Context, schedule FeeSchedule, actorID string, at time.Time) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE transaction_fees
SET flat = ?::numeric, markup_bps = ?, updated_by = ?, updated_at = ?
WHERE kind = ? AND currency = ?
`, schedule.Flat, schedule.MarkupBps, actorID, at, schedule.Kind, schedule.Currency).Error
}

func (r *Repository) ListCrew(ctx context.Context, organisationID string) ([]CrewMember, error) {
	var rows []struct {
		UserID    string
		FirstName string
		LastName  string
		Email     string
		Phone     string
		Status    string
		Role      string
		Currency  string
		Balance   string
		Username  string
	}
	err := r.db.WithContext(ctx).Table("organisation_members").
		Select(`users.id AS user_id, users.first_name, users.last_name, users.email, COALESCE(users.phone, '') AS phone, users.status, organisation_members.member_role AS role, COALESCE(wallets.currency, '') AS currency, COALESCE(wallets.balance::text, '') AS balance, COALESCE(wallets.username, '') AS username`).
		Joins("JOIN users ON users.id = organisation_members.user_id").
		Joins("LEFT JOIN wallets ON wallets.user_id = users.id AND wallets.kind = 'personal' AND wallets.currency = 'UGX'").
		Where("organisation_members.organisation_id = ? AND organisation_members.member_role IN ?", organisationID, []string{auth.RoleGuide, auth.RoleDriver}).
		Order("organisation_members.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	crew := make([]CrewMember, 0, len(rows))
	for _, row := range rows {
		crew = append(crew, CrewMember{
			UserID: row.UserID, Name: stringsTrim(row.FirstName, row.LastName), Email: row.Email, Phone: row.Phone,
			Role: row.Role, Status: row.Status, WalletCurrency: row.Currency, WalletBalance: row.Balance, Username: row.Username,
		})
	}
	return crew, nil
}

func (r *Repository) usernameBase(ctx context.Context, db *gorm.DB, wallet Wallet) string {
	if strings.TrimSpace(wallet.Name) != "" {
		return wallet.Name
	}
	if wallet.UserID != "" {
		var row struct {
			FirstName string
			LastName  string
		}
		_ = db.WithContext(ctx).Table("users").Select("first_name, last_name").Where("id = ?", wallet.UserID).Scan(&row).Error
		if name := strings.TrimSpace(row.FirstName + " " + row.LastName); name != "" {
			return name
		}
	}
	if wallet.Organisation != "" {
		var name string
		_ = db.WithContext(ctx).Table("organisations").Select("name").Where("id = ?", wallet.Organisation).Scan(&name).Error
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	if wallet.Kind != "" {
		return wallet.Kind
	}
	return "wallet"
}

func (r *Repository) nextUsername(ctx context.Context, db *gorm.DB, base, currency string) (string, error) {
	for n := 1; n < 100; n++ {
		candidate := usernameCandidate(base, currency, n)
		var count int64
		if err := db.WithContext(ctx).Model(&walletRow{}).Where("username = ?", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("username")
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (Wallet, string, error) {
	var row struct {
		ID             string
		OrganisationID *string
		UserID         *string
		Kind           string
		Name           string
		Username       string
		Currency       string
		Balance        string
		IsDefault      bool
		OwnerName      string
	}
	err := r.db.WithContext(ctx).Table("wallets").
		Select(`wallets.id, wallets.organisation_id, wallets.user_id, wallets.kind, wallets.name, wallets.username, wallets.currency, wallets.balance::text AS balance, wallets.is_default, COALESCE(NULLIF(trim(concat_ws(' ', users.first_name, users.last_name)), ''), NULLIF(trim(organisations.name), ''), wallets.name) AS owner_name`).
		Joins("LEFT JOIN users ON users.id = wallets.user_id").
		Joins("LEFT JOIN organisations ON organisations.id = wallets.organisation_id").
		Where("wallets.username = ?", username).
		Scan(&row).Error
	if err != nil {
		return Wallet{}, "", err
	}
	if row.ID == "" {
		return Wallet{}, "", errNotFound
	}
	wallet := Wallet{
		ID: row.ID, Kind: row.Kind, Name: row.Name, Username: row.Username, Currency: row.Currency, Balance: row.Balance, Default: row.IsDefault,
	}
	if row.UserID != nil {
		wallet.UserID = *row.UserID
	}
	if row.OrganisationID != nil {
		wallet.Organisation = *row.OrganisationID
	}
	return wallet, strings.TrimSpace(row.OwnerName), nil
}

func (r *Repository) UserRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := r.db.WithContext(ctx).Table("users").Select("role").Where("id = ?", userID).Scan(&role).Error
	if err != nil {
		return "", err
	}
	if role == "" {
		return "", errNotFound
	}
	return role, nil
}
