package company

import (
	"context"
	"math/big"
	"testing"
	"time"
	"virungapay-be/internal/middleware"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
)

func TestOperatorSignupDoesNotCreateCompany(t *testing.T) {
	reg := &memRegistrar{}
	store := newMemCompany()
	svc := NewService(store, reg, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")

	user, err := svc.RegisterOperator(context.Background(), auth.Signup{
		FirstName: "Grace", LastName: "Kato", Email: "grace@example.com", Password: "password1",
	})
	require.NoError(t, err)
	require.Equal(t, auth.RoleOperatorOwner, user.Role)
	require.Empty(t, store.members)
	require.True(t, reg.operator)
}

func TestInviteSendsMailAndTouristCannotInvite(t *testing.T) {
	reg := &memRegistrar{}
	store := newMemCompany()
	box := &mail.Recorder{}
	svc := NewService(store, reg, box, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	_, err := svc.Invite(ctx, "tourist-1", auth.RoleTourist, InviteInput{Name: "Peter", Email: "peter@example.com", Role: auth.RoleDriver})
	require.Error(t, err)
	var app *middleware.AppError
	require.ErrorAs(t, err, &app)
	require.Equal(t, "forbidden", app.Code)
	require.Empty(t, box.Messages)

	store.members["owner-1"] = Member{OrganisationID: "org-1", UserID: "owner-1", Role: auth.RoleOperatorOwner}
	invited, err := svc.Invite(ctx, "owner-1", auth.RoleOperatorOwner, InviteInput{
		Name: "Peter", Email: "peter@example.com", Role: auth.RoleDriver,
	})
	require.NoError(t, err)
	require.Equal(t, auth.StatusInvited, invited.Status)
	require.Equal(t, auth.RoleDriver, invited.Role)
	require.NotContains(t, invited.Email, "token")
	require.Len(t, box.Messages, 1)
	require.NotEmpty(t, box.TokenFor("peter@example.com"))
	wallet, ok := store.personal["crew-1/UGX"]
	require.True(t, ok)
	require.Equal(t, "personal", wallet.Kind)
	require.Equal(t, "0.00", wallet.Balance)
	require.Equal(t, "crew-1", wallet.UserID)
	require.Equal(t, "peter-ugx", wallet.Username)
}

type memCompany struct {
	members   map[string]Member
	orgs      map[string]Company
	wallets   map[string][]Wallet
	tariffs   map[string]Tariff
	fees      map[string]FeeSchedule
	platform  map[string]Wallet
	personal  map[string]Wallet
	movements []Movement
	roles     map[string]string
}

func newMemCompany() *memCompany {
	return &memCompany{members: map[string]Member{}, orgs: map[string]Company{}, wallets: map[string][]Wallet{}}
}

func (m *memCompany) Member(_ context.Context, _ *gorm.DB, userID string) (Member, error) {
	member, ok := m.members[userID]
	if !ok {
		return Member{}, errNotFound
	}
	return member, nil
}

func (m *memCompany) Company(_ context.Context, organisationID string) (Company, error) {
	company, ok := m.orgs[organisationID]
	if !ok {
		return Company{}, errNotFound
	}
	return company, nil
}

func (m *memCompany) InsertCompany(_ context.Context, _ *gorm.DB, org Organisation, member Member) error {
	m.members[member.UserID] = member
	m.orgs[org.ID] = Company{ID: org.ID, Name: org.Name, Country: org.Country, AssociationMemberOf: org.AssociationMemberOf, Status: org.Status, Type: org.Type, Wallets: []Wallet{}}
	return nil
}

func (m *memCompany) SaveMember(_ context.Context, member Member) error {
	m.members[member.UserID] = member
	return nil
}

func (m *memCompany) SaveInvite(context.Context, Invite) error { return nil }

func (m *memCompany) Wallets(_ context.Context, _ *gorm.DB, organisationID string) ([]Wallet, error) {
	wallets := m.wallets[organisationID]
	if wallets == nil {
		return []Wallet{}, nil
	}
	copied := append([]Wallet{}, wallets...)
	return copied, nil
}

func (m *memCompany) InsertWallet(_ context.Context, _ *gorm.DB, wallet Wallet, _ time.Time) error {
	if wallet.Balance == "" {
		wallet.Balance = "0.00"
	}
	if wallet.Username == "" {
		base := wallet.Name
		if base == "" {
			base = wallet.Kind
		}
		wallet.Username = usernameCandidate(base, wallet.Currency, 1)
	}
	if wallet.UserID != "" {
		if m.personal == nil {
			m.personal = map[string]Wallet{}
		}
		m.personal[wallet.UserID+"/"+wallet.Currency] = wallet
		return nil
	}
	if wallet.Organisation == "" {
		if m.platform == nil {
			m.platform = map[string]Wallet{}
		}
		m.platform[wallet.Kind+"/"+wallet.Currency] = wallet
		return nil
	}
	m.wallets[wallet.Organisation] = append(m.wallets[wallet.Organisation], wallet)
	return nil
}

func (m *memCompany) SetStatus(_ context.Context, _ *gorm.DB, organisationID, status string, _ time.Time) error {
	org := m.orgs[organisationID]
	org.Status = status
	m.orgs[organisationID] = org
	return nil
}

func (m *memCompany) ClearDefaults(_ context.Context, _ *gorm.DB, organisationID string, _ time.Time) error {
	wallets := m.wallets[organisationID]
	for i := range wallets {
		wallets[i].Default = false
	}
	m.wallets[organisationID] = wallets
	return nil
}

func (m *memCompany) WalletByID(_ context.Context, walletID string) (Wallet, error) {
	for _, wallets := range m.wallets {
		for _, wallet := range wallets {
			if wallet.ID == walletID {
				return wallet, nil
			}
		}
	}
	for _, wallet := range m.personal {
		if wallet.ID == walletID {
			return wallet, nil
		}
	}
	for _, wallet := range m.platform {
		if wallet.ID == walletID {
			return wallet, nil
		}
	}
	return Wallet{}, errNotFound
}

func (m *memCompany) RenameWallet(_ context.Context, walletID, name, username string, _ time.Time) error {
	for orgID, wallets := range m.wallets {
		for i := range wallets {
			if wallets[i].ID == walletID {
				wallets[i].Name = name
				wallets[i].Username = username
				m.wallets[orgID] = wallets
				return nil
			}
		}
	}
	for key, wallet := range m.personal {
		if wallet.ID == walletID {
			wallet.Name = name
			wallet.Username = username
			m.personal[key] = wallet
			return nil
		}
	}
	return errNotFound
}

func (m *memCompany) MarkDefault(_ context.Context, _ *gorm.DB, walletID string, _ time.Time) error {
	for orgID, wallets := range m.wallets {
		for i := range wallets {
			if wallets[i].ID == walletID {
				wallets[i].Default = true
				m.wallets[orgID] = wallets
				return nil
			}
		}
	}
	return nil
}

func (m *memCompany) ListBusinesses(context.Context) ([]Business, error) {
	businesses := []Business{}
	for _, org := range m.orgs {
		wallets := append([]Wallet{}, m.wallets[org.ID]...)
		businesses = append(businesses, Business{
			ID: org.ID, Name: org.Name, Country: org.Country, AssociationMemberOf: org.AssociationMemberOf,
			Status: org.Status, Wallets: wallets,
		})
	}
	return businesses, nil
}

func (m *memCompany) Tariffs(context.Context) ([]Tariff, error) {
	tariffs := []Tariff{}
	for _, tariff := range m.tariffs {
		tariffs = append(tariffs, tariff)
	}
	return tariffs, nil
}

func (m *memCompany) SaveTariff(_ context.Context, tariff Tariff, _ string, _ time.Time) error {
	if m.tariffs == nil {
		m.tariffs = map[string]Tariff{}
	}
	m.tariffs[tariff.Pair] = tariff
	return nil
}

func (m *memCompany) DB() *gorm.DB { return nil }

func (m *memCompany) ListBureaus(context.Context) ([]Business, error) { return []Business{}, nil }

func (m *memCompany) ListFeeSchedules(context.Context) ([]FeeSchedule, error) {
	schedules := []FeeSchedule{}
	for _, schedule := range m.fees {
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (m *memCompany) FeeSchedule(_ context.Context, _ *gorm.DB, kind, currency string) (FeeSchedule, error) {
	schedule, ok := m.fees[kind+"/"+currency]
	if !ok {
		return FeeSchedule{}, errNotFound
	}
	return schedule, nil
}

func (m *memCompany) SaveFeeSchedule(_ context.Context, schedule FeeSchedule, _ string, _ time.Time) error {
	if m.fees == nil {
		m.fees = map[string]FeeSchedule{}
	}
	m.fees[schedule.Kind+"/"+schedule.Currency] = schedule
	return nil
}

func (m *memCompany) ListPersonWallets(context.Context) ([]PersonWallet, error) { return nil, nil }
func (m *memCompany) ListTourists(context.Context) ([]TouristAccount, error)    { return nil, nil }
func (m *memCompany) ListPlatformWallets(context.Context) ([]Wallet, error) {
	wallets := []Wallet{}
	for _, wallet := range m.platform {
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}

func (m *memCompany) PlatformWallet(_ context.Context, _ *gorm.DB, kind, currency string) (Wallet, error) {
	wallet, ok := m.platform[kind+"/"+currency]
	if !ok {
		return Wallet{}, errNotFound
	}
	return wallet, nil
}

func (m *memCompany) PersonalWallet(_ context.Context, _ *gorm.DB, userID, currency string) (Wallet, error) {
	wallet, ok := m.personal[userID+"/"+currency]
	if !ok {
		return Wallet{}, errNotFound
	}
	return wallet, nil
}

func (m *memCompany) PersonalWallets(_ context.Context, userID string) ([]Wallet, error) {
	wallets := []Wallet{}
	prefix := userID + "/"
	for key, wallet := range m.personal {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			wallets = append(wallets, wallet)
		}
	}
	return wallets, nil
}

func (m *memCompany) AdjustBalance(_ context.Context, _ *gorm.DB, walletID, delta string, _ time.Time) error {
	change, ok := new(big.Rat).SetString(delta)
	if !ok {
		return errNotFound
	}
	update := func(wallet *Wallet) bool {
		if wallet.ID != walletID {
			return false
		}
		current, _ := new(big.Rat).SetString(wallet.Balance)
		if current == nil {
			current = new(big.Rat)
		}
		next := new(big.Rat).Add(current, change)
		if next.Sign() < 0 {
			return false
		}
		wallet.Balance = next.FloatString(2)
		return true
	}
	for id, wallets := range m.wallets {
		for i := range wallets {
			if update(&wallets[i]) {
				m.wallets[id] = wallets
				return nil
			}
		}
	}
	for key, wallet := range m.platform {
		if update(&wallet) {
			m.platform[key] = wallet
			return nil
		}
	}
	for key, wallet := range m.personal {
		if update(&wallet) {
			m.personal[key] = wallet
			return nil
		}
	}
	return errNotFound
}

func (m *memCompany) PlatformWalletByID(_ context.Context, walletID string) (Wallet, error) {
	for _, wallet := range m.platform {
		if wallet.ID == walletID {
			return wallet, nil
		}
	}
	return Wallet{}, errNotFound
}

func (m *memCompany) ListMovements(_ context.Context, walletID string) ([]Movement, error) {
	movements := []Movement{}
	for _, movement := range m.movements {
		if movement.WalletID == walletID {
			movements = append(movements, movement)
		}
	}
	return movements, nil
}

func (m *memCompany) InsertMovement(_ context.Context, _ *gorm.DB, movement Movement) error {
	m.movements = append(m.movements, movement)
	return nil
}

func (m *memCompany) InsertTransaction(context.Context, *gorm.DB, TransactionRecord) error {
	return nil
}

func (m *memCompany) ListTransactions(context.Context) ([]Transaction, error) {
	return nil, nil
}

func (m *memCompany) ListWalletActivity(context.Context, string) ([]WalletActivity, error) {
	return nil, nil
}

func (m *memCompany) ListPersonalActivity(context.Context, string) ([]WalletActivity, error) {
	return nil, nil
}

func (m *memCompany) FindByUsername(_ context.Context, username string) (Wallet, string, error) {
	for _, wallet := range m.personal {
		if wallet.Username == username {
			return wallet, wallet.Name, nil
		}
	}
	for _, wallets := range m.wallets {
		for _, wallet := range wallets {
			if wallet.Username == username {
				return wallet, wallet.Name, nil
			}
		}
	}
	for _, wallet := range m.platform {
		if wallet.Username == username {
			return wallet, wallet.Name, nil
		}
	}
	return Wallet{}, "", errNotFound
}

func (m *memCompany) ListCrew(_ context.Context, organisationID string) ([]CrewMember, error) {
	crew := make([]CrewMember, 0)
	for _, member := range m.members {
		if member.OrganisationID != organisationID || (member.Role != auth.RoleGuide && member.Role != auth.RoleDriver) {
			continue
		}
		wallet := m.personal[member.UserID+"/UGX"]
		crew = append(crew, CrewMember{
			UserID: member.UserID, Role: member.Role, WalletCurrency: wallet.Currency, WalletBalance: wallet.Balance,
		})
	}
	return crew, nil
}

func (m *memCompany) UserRole(_ context.Context, userID string) (string, error) {
	role, ok := m.roles[userID]
	if !ok {
		return "", errNotFound
	}
	return role, nil
}

type memRegistrar struct {
	operator bool
}

func (m *memRegistrar) RegisterOperator(_ context.Context, in auth.Signup) (auth.User, error) {
	m.operator = true
	return auth.User{ID: "owner-1", Role: auth.RoleOperatorOwner, FirstName: in.FirstName, LastName: in.LastName, Email: in.Email, Status: auth.StatusActive}, nil
}

func (m *memRegistrar) RegisterForexBureau(_ context.Context, in auth.Signup) (auth.User, error) {
	return auth.User{ID: "bureau-owner-1", Role: auth.RoleForexBureauOwner, FirstName: in.FirstName, LastName: in.LastName, Email: in.Email, Status: auth.StatusActive}, nil
}

func (m *memRegistrar) CreateInvitedUser(_ context.Context, name, email, _, role string) (auth.User, string, string, error) {
	return auth.User{ID: "crew-1", Role: role, FirstName: name, Email: email, Status: auth.StatusInvited}, "raw-token", "hashed-token", nil
}

func TestTouristStatementKeepsARunningBalance(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	wallets, err := svc.MyWallets(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	store.movements = []Movement{
		{ID: "in-1", WalletID: wallets[0].ID, Kind: "p2p", Direction: "in", Amount: "1000.00", Currency: "UGX", CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)},
		{ID: "out-1", WalletID: wallets[0].ID, Kind: "p2p", Direction: "out", Amount: "200.00", Currency: "UGX", CreatedAt: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)},
	}

	payments, err := svc.ListOperatorPayments(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.Empty(t, payments)

	wallet, lines, err := svc.WalletStatement(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.Equal(t, "VirungaPass Wallet", wallet.Name)
	require.Equal(t, "UGX", wallet.Currency)
	require.Len(t, lines, 2)
	require.Equal(t, "out", lines[0].Direction)
	require.Equal(t, "800.00", lines[0].RunningBalance)
	require.Equal(t, "1000.00", lines[1].RunningBalance)

	_, _, err = svc.WalletStatement(ctx, "guide-1", auth.RoleGuide)
	require.Error(t, err)
}

func TestOperatorStatementCoversEachWallet(t *testing.T) {
	store := newMemCompany()
	store.members["owner-1"] = Member{OrganisationID: "org-1", UserID: "owner-1", Role: auth.RoleOperatorOwner}
	store.orgs["org-1"] = Company{ID: "org-1", Name: "Kigezi Highland Safaris", Status: "approved", Type: "operator"}
	store.wallets["org-1"] = []Wallet{
		{ID: "usd", Name: "Dollar float", Currency: "USD", Balance: "5.00", Organisation: "org-1"},
		{ID: "ugx", Name: "Shilling float", Currency: "UGX", Balance: "100.00", Organisation: "org-1"},
	}
	store.movements = []Movement{
		{ID: "m1", WalletID: "ugx", Kind: "collection", Direction: "in", Amount: "100.00", Currency: "UGX", CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)},
	}
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	name, wallets, groups, err := svc.CompanyStatement(ctx, "owner-1", auth.RoleOperatorOwner)
	require.NoError(t, err)
	require.Equal(t, "Kigezi Highland Safaris", name)
	require.Equal(t, []string{"UGX", "USD"}, []string{wallets[0].Currency, wallets[1].Currency})
	require.Len(t, groups[0], 1)
	require.Equal(t, "100.00", groups[0][0].RunningBalance)
	require.Empty(t, groups[1])
	_, _, _, err = svc.CompanyStatement(ctx, "tourist-1", auth.RoleTourist)
	require.Error(t, err)

	chosen, lines, err := svc.Statement(ctx, "owner-1", auth.RoleOperatorOwner, "ugx")
	require.NoError(t, err)
	require.Equal(t, "UGX", chosen.Currency)
	require.Len(t, lines, 1)
	require.Equal(t, "100.00", lines[0].RunningBalance)
	_, _, err = svc.Statement(ctx, "tourist-1", auth.RoleTourist, "ugx")
	require.Error(t, err)
}

func TestTouristGetsOneVirungaPassWallet(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	first, err := svc.MyWallets(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, "VirungaPass Wallet", first[0].Name)
	require.Equal(t, "UGX", first[0].Currency)

	second, err := svc.MyWallets(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].ID, second[0].ID)
	require.Len(t, store.personal, 1)
}

func TestTouristLocalCurrencyIsUgandanForNow(t *testing.T) {
	currency, ok := TouristLocalCurrency("")
	require.True(t, ok)
	require.Equal(t, "UGX", currency)
	currency, ok = TouristLocalCurrency("ug")
	require.True(t, ok)
	require.Equal(t, "UGX", currency)
	_, ok = TouristLocalCurrency("KE")
	require.False(t, ok)
}

func TestTipUsesTheVirungaPassWallet(t *testing.T) {
	store := newMemCompany()
	store.roles = map[string]string{"tourist-1": auth.RoleTourist, "guide-1": auth.RoleGuide}
	store.personal = map[string]Wallet{
		"tourist-1/UGX": {ID: "pass-1", Kind: "virungapass", Name: "VirungaPass Wallet", Currency: "UGX", Balance: "5000.00", UserID: "tourist-1"},
		"guide-1/UGX":   {ID: "guide-1", Kind: "personal", Currency: "UGX", Balance: "0.00", UserID: "guide-1"},
	}
	store.fees = map[string]FeeSchedule{"p2p/UGX": {Kind: "p2p", Currency: "UGX", Flat: "0.00"}}
	store.platform = map[string]Wallet{"fee_transaction/UGX": {ID: "fee-ugx", Kind: "fee_transaction", Currency: "UGX", Balance: "0.00"}}
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	err := svc.RecordTip(ctx, auth.RoleVASuperadmin, "tourist-1", "guide-1", "GBP", "10")
	require.Error(t, err)
	_, exists := store.personal["tourist-1/GBP"]
	require.False(t, exists)

	require.NoError(t, svc.RecordTip(ctx, auth.RoleVASuperadmin, "tourist-1", "guide-1", "UGX", "1000"))
	require.Equal(t, "4000.00", store.personal["tourist-1/UGX"].Balance)
	require.Equal(t, "1000.00", store.personal["guide-1/UGX"].Balance)
}

func TestGuideSeesPersonalWalletsAndTouristCannotWithdraw(t *testing.T) {
	store := newMemCompany()
	store.personal = map[string]Wallet{
		"guide-1/UGX": {ID: "wallet-ugx", Currency: "UGX", Balance: "40000.00", UserID: "guide-1"},
	}
	store.orgs["bureau-1"] = Company{ID: "bureau-1", Name: "Kampala Forex", Type: "forex_bureau", Status: "pending"}
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	wallets, err := svc.MyWallets(ctx, "guide-1", auth.RoleGuide)
	require.NoError(t, err)
	require.Equal(t, "UGX", wallets[0].Currency)

	_, err = svc.MyWallets(ctx, "owner-1", auth.RoleOperatorOwner)
	require.Error(t, err)

	err = svc.WithdrawOwn(ctx, "tourist-1", auth.RoleTourist, "bureau-1", "UGX", "1000")
	require.Error(t, err)

	store.personal["tourist-1/UGX"] = Wallet{ID: "pass-1", Kind: "virungapass", Name: "VirungaPass Wallet", Currency: "UGX", Balance: "0.00", UserID: "tourist-1"}
	store.personal["tourist-1/USD"] = Wallet{ID: "other-1", Kind: "personal", Currency: "USD", Balance: "1.00", UserID: "tourist-1"}
	touristWallets, err := svc.MyWallets(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.Len(t, touristWallets, 1)
	require.Equal(t, "VirungaPass Wallet", touristWallets[0].Name)
	require.Equal(t, "UGX", touristWallets[0].Currency)

	list, err := svc.ApprovedBureaus(ctx, auth.RoleGuide)
	require.NoError(t, err)
	require.Empty(t, list.Businesses)
}

func TestOwnerRenamesWalletWithoutChangingUsername(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	created, err := svc.Create(ctx, "owner-1", auth.RoleOperatorOwner, CreateInput{Name: "Kigezi", Country: "UG"})
	require.NoError(t, err)
	_, err = svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	company, err := svc.Mine(ctx, "owner-1", auth.RoleOperatorOwner)
	require.NoError(t, err)
	require.NotEmpty(t, company.Wallets)
	wallet := company.Wallets[0]

	renamed, err := svc.RenameWallet(ctx, "owner-1", auth.RoleOperatorOwner, wallet.ID, "  Safari float  ", "")
	require.NoError(t, err)
	require.Equal(t, "Safari float", renamed.Name)
	require.Equal(t, wallet.Username, renamed.Username)

	renamed, err = svc.RenameWallet(ctx, "owner-1", auth.RoleOperatorOwner, wallet.ID, "Safari float", "Kigezi Cash")
	require.NoError(t, err)
	require.Equal(t, "kigezi-cash", renamed.Username)

	again, err := svc.Mine(ctx, "owner-1", auth.RoleOperatorOwner)
	require.NoError(t, err)
	require.Equal(t, "Safari float", again.Wallets[0].Name)
	require.Equal(t, "kigezi-cash", again.Wallets[0].Username)

	_, err = svc.RenameWallet(ctx, "tourist-1", auth.RoleTourist, wallet.ID, "Not mine", "")
	require.Error(t, err)
	_, err = svc.RenameWallet(ctx, "owner-1", auth.RoleOperatorOwner, wallet.ID, "   ", "")
	require.Error(t, err)
	_, err = svc.RenameWallet(ctx, "owner-1", auth.RoleOperatorOwner, wallet.ID, "Safari float", "ab")
	require.Error(t, err)

	tourist, err := svc.MyWallets(ctx, "tourist-1", auth.RoleTourist)
	require.NoError(t, err)
	require.NotEmpty(t, tourist)
	_, err = svc.RenameWallet(ctx, "tourist-1", auth.RoleTourist, tourist[0].ID, "Park money", "kigezi-cash")
	require.Error(t, err)
	renamedPass, err := svc.RenameWallet(ctx, "tourist-1", auth.RoleTourist, tourist[0].ID, "Park money", "@ParkMoney")
	require.NoError(t, err)
	require.Equal(t, "VirungaPass Wallet", renamedPass.Name)
	require.Equal(t, "parkmoney", renamedPass.Username)
}

func TestTouristCanTopUpSendMobileMoneyAndTip(t *testing.T) {
	store := newMemCompany()
	store.roles = map[string]string{"tourist-1": auth.RoleTourist, "guide-1": auth.RoleGuide, "owner-1": auth.RoleOperatorOwner}
	store.personal = map[string]Wallet{
		"tourist-1/UGX": {ID: "pass-1", Kind: "virungapass", Name: "VirungaPass Wallet", Username: "amina-ugx", Currency: "UGX", Balance: "0.00", UserID: "tourist-1"},
		"guide-1/UGX":   {ID: "guide-wallet", Kind: "personal", Name: "Peter", Username: "peter-ugx", Currency: "UGX", Balance: "0.00", UserID: "guide-1"},
	}
	store.platform = map[string]Wallet{
		"ova/UGX":             {ID: "ova-ugx", Kind: "ova", Currency: "UGX", Balance: "0.00"},
		"fee_transaction/UGX": {ID: "fee-ugx", Kind: "fee_transaction", Currency: "UGX", Balance: "0.00"},
	}
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	require.NoError(t, svc.TopUpCard(ctx, "tourist-1", auth.RoleTourist, "10000", "4242"))
	require.Equal(t, "10000.00", store.personal["tourist-1/UGX"].Balance)
	require.Error(t, svc.TopUpCard(ctx, "owner-1", auth.RoleOperatorOwner, "1000", "4242"))
	require.Error(t, svc.SendMobileMoney(ctx, "tourist-1", auth.RoleTourist, "airtel", "+254700000000", "1000"))
	require.NoError(t, svc.SendMobileMoney(ctx, "tourist-1", auth.RoleTourist, "mtn", "0771234567", "2000"))
	require.Equal(t, "8000.00", store.personal["tourist-1/UGX"].Balance)

	require.Error(t, svc.TipCrew(ctx, "tourist-1", auth.RoleTourist, "amina-ugx", "500", "tip"))
	require.NoError(t, svc.TipCrew(ctx, "tourist-1", auth.RoleTourist, "@peter-ugx", "1500", ""))
	require.Equal(t, "6500.00", store.personal["tourist-1/UGX"].Balance)
	require.Equal(t, "1500.00", store.personal["guide-1/UGX"].Balance)

	store.orgs["org-1"] = Company{ID: "org-1", Name: "Kigezi Highland Safaris", Status: "approved", Type: "operator"}
	store.wallets["org-1"] = []Wallet{{
		ID: "op-ugx", Kind: "operator", Name: "Kigezi", Username: "kigezi-ugx", Currency: "UGX", Balance: "0.00", Organisation: "org-1",
	}}
	require.Error(t, svc.TipCrew(ctx, "tourist-1", auth.RoleTourist, "kigezi-ugx", "500", "tip"))
	require.Error(t, svc.TipCrew(ctx, "tourist-1", auth.RoleTourist, "peter-ugx", "500", "service"))
	require.NoError(t, svc.TipCrew(ctx, "tourist-1", auth.RoleTourist, "kigezi-ugx", "1000", "service"))
	require.Equal(t, "5500.00", store.personal["tourist-1/UGX"].Balance)
	require.Equal(t, "1000.00", store.wallets["org-1"][0].Balance)
}
