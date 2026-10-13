package company

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
)

func TestApproveOpensUGXAndUSDCanBecomeDefault(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	created, err := svc.Create(ctx, "owner-1", auth.RoleOperatorOwner, CreateInput{Name: "Kigezi Highland Safaris", Country: "ug"})
	require.NoError(t, err)
	require.Equal(t, "pending", created.Status)

	approved, err := svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.Len(t, approved.Wallets, 1)
	require.Equal(t, "UGX", approved.Wallets[0].Currency)
	require.True(t, approved.Wallets[0].Default)

	again, err := svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	require.Len(t, again.Wallets, 1)

	withUSD, err := svc.OpenWallet(ctx, "staff-1", auth.RoleVASuperadmin, created.ID, "USD")
	require.NoError(t, err)
	require.Len(t, withUSD.Wallets, 2)

	var usdID string
	for _, wallet := range withUSD.Wallets {
		if wallet.Currency == "USD" {
			usdID = wallet.ID
			require.False(t, wallet.Default)
		}
	}
	stopped, err := svc.Disable(ctx, auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", stopped.Status)
	restored, err := svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", restored.Status)
	require.Len(t, restored.Wallets, 2)

	switched, err := svc.SetDefaultWallet(ctx, auth.RoleVASuperadmin, created.ID, usdID)
	require.NoError(t, err)
	for _, wallet := range switched.Wallets {
		require.Equal(t, wallet.Currency == "USD", wallet.Default)
	}
}

func TestOperatorCanRequestAEurWallet(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	created, err := svc.Create(ctx, "owner-1", auth.RoleOperatorOwner, CreateInput{Name: "Jims Travels", Country: "UG"})
	require.NoError(t, err)
	_, err = svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)

	_, err = svc.RequestWallet(ctx, "owner-1", auth.RoleGuide, "EUR")
	require.Error(t, err)

	opened, err := svc.RequestWallet(ctx, "owner-1", auth.RoleOperatorOwner, "EUR")
	require.NoError(t, err)
	var currencies []string
	for _, wallet := range opened.Wallets {
		currencies = append(currencies, wallet.Currency)
	}
	require.ElementsMatch(t, []string{"UGX", "EUR"}, currencies)

	_, err = svc.RequestWallet(ctx, "owner-1", auth.RoleOperatorOwner, "EUR")
	require.Error(t, err)
	_, err = svc.RequestWallet(ctx, "owner-1", auth.RoleOperatorOwner, "USD")
	require.Error(t, err)
}

func TestOnlyStaffCanSetTheDaysTariff(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	_, err := svc.SetTariff(ctx, "owner-1", auth.RoleOperatorOwner, "USD/UGX", "3700", 100)
	require.Error(t, err)

	tariff, err := svc.SetTariff(ctx, "staff-1", auth.RoleVAFinance, "USD/UGX", "3700", 100)
	require.NoError(t, err)
	require.Equal(t, "USD/UGX", tariff.Pair)
	require.Equal(t, "3737.00000000", tariff.CustomerRate)

	euro, err := svc.SetTariff(ctx, "staff-1", auth.RoleVAFinance, "EUR/UGX", "4000", 50)
	require.NoError(t, err)
	require.Equal(t, "4020.00000000", euro.CustomerRate)

	stored := store.tariffs["USD/UGX"]
	replaced, err := svc.SetTariff(ctx, "staff-1", auth.RoleVAFinance, "USD/UGX", "3800", 0)
	require.NoError(t, err)
	require.Equal(t, stored.QuoteOn, replaced.QuoteOn)
	require.Equal(t, "3800.00000000", replaced.CustomerRate)
	require.Equal(t, "4020.00000000", store.tariffs["EUR/UGX"].CustomerRate)

	listed, err := svc.TodayTariffs(ctx, auth.RoleOperatorOwner)
	require.NoError(t, err)
	require.Equal(t, []string{"USD/UGX", "EUR/UGX"}, []string{listed[0].Pair, listed[1].Pair})

	older := store.tariffs["EUR/UGX"]
	older.QuoteOn = "2020-01-01"
	store.tariffs["EUR/UGX"] = older
	listed, err = svc.TodayTariffs(ctx, auth.RoleOperatorOwner)
	require.NoError(t, err)
	require.Equal(t, "2020-01-01", listed[1].QuoteOn)
	require.Equal(t, "4020.00000000", listed[1].CustomerRate)
}

func TestStaffCanAddAForexBureau(t *testing.T) {
	store := newMemCompany()
	box := &mail.Recorder{}
	svc := NewService(store, &memRegistrar{}, box, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()

	_, err := svc.AddBureau(ctx, auth.RoleOperatorOwner, "Kampala Forex", "UG", "Amina Kato", "amina@example.com", "")
	require.Error(t, err)

	created, err := svc.AddBureau(ctx, auth.RoleVAFinance, "Kampala Forex", "UG", "Amina Kato", "amina@example.com", "+256700000000")
	require.NoError(t, err)
	require.Equal(t, "pending", created.Status)
	require.Equal(t, "Kampala Forex", created.Name)
	require.Equal(t, "UG", created.Country)
	require.Equal(t, "amina@example.com", created.OwnerEmail)
	require.Equal(t, "forex_bureau", store.orgs[created.ID].Type)
	require.Contains(t, box.Messages[0].Text, "/invites/accept?token=")
	require.Equal(t, "amina@example.com", box.Messages[0].ToEmail)
}
