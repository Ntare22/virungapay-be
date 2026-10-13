package company

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/mail"
)

func TestCollectionSplitsTheAmountAcrossWallets(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	created, err := svc.Create(ctx, "owner-1", auth.RoleOperatorOwner, CreateInput{Name: "Kigezi", Country: "UG"})
	require.NoError(t, err)
	_, err = svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	_, err = svc.OpenWallet(ctx, "staff-1", auth.RoleVASuperadmin, created.ID, "USD")
	require.NoError(t, err)
	require.NoError(t, store.InsertWallet(ctx, nil, Wallet{ID: "ova-usd", Kind: "ova", Currency: "USD", Balance: "0.00"}, svc.now()))
	require.NoError(t, store.InsertWallet(ctx, nil, Wallet{ID: "fee-usd", Kind: "fee_transaction", Currency: "USD", Balance: "0.00"}, svc.now()))
	require.NoError(t, store.SaveFeeSchedule(ctx, FeeSchedule{Kind: "collection", Currency: "USD", Flat: "2.00", MarkupBps: 0}, "staff-1", svc.now()))

	require.NoError(t, svc.RecordCollection(ctx, auth.RoleVAFinance, created.ID, "USD", "100"))

	ova, err := store.PlatformWallet(ctx, nil, "ova", "USD")
	require.NoError(t, err)
	require.Equal(t, "100.00", ova.Balance)
	fee, err := store.PlatformWallet(ctx, nil, "fee_transaction", "USD")
	require.NoError(t, err)
	require.Equal(t, "2.00", fee.Balance)
	wallets, err := store.Wallets(ctx, nil, created.ID)
	require.NoError(t, err)
	var usd string
	for _, wallet := range wallets {
		if wallet.Currency == "USD" {
			usd = wallet.Balance
		}
	}
	require.Equal(t, "98.00", usd)

	_, lines, err := svc.PlatformStatement(ctx, auth.RoleVAFinance, "ova-usd")
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, "in", lines[0].Direction)
	require.Equal(t, "100.00", lines[0].Amount)
	require.Equal(t, "100.00", lines[0].RunningBalance)
}

func TestMoveBetweenOperatorWalletsUsesTheStandingRate(t *testing.T) {
	store := newMemCompany()
	svc := NewService(store, &memRegistrar{}, &mail.Recorder{}, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	created, err := svc.Create(ctx, "owner-1", auth.RoleOperatorOwner, CreateInput{Name: "Kigezi", Country: "UG"})
	require.NoError(t, err)
	_, err = svc.Approve(ctx, "staff-1", auth.RoleVASuperadmin, created.ID)
	require.NoError(t, err)
	_, err = svc.OpenWallet(ctx, "staff-1", auth.RoleVASuperadmin, created.ID, "USD")
	require.NoError(t, err)
	_, err = svc.OpenWallet(ctx, "staff-1", auth.RoleVASuperadmin, created.ID, "EUR")
	require.NoError(t, err)
	require.NoError(t, store.InsertWallet(ctx, nil, Wallet{ID: "ova-usd", Kind: "ova", Currency: "USD", Balance: "0.00"}, svc.now()))
	require.NoError(t, store.InsertWallet(ctx, nil, Wallet{ID: "ova-ugx", Kind: "ova", Currency: "UGX", Balance: "0.00"}, svc.now()))
	require.NoError(t, store.InsertWallet(ctx, nil, Wallet{ID: "fee-usd", Kind: "fee_transaction", Currency: "USD", Balance: "0.00"}, svc.now()))
	require.NoError(t, store.SaveFeeSchedule(ctx, FeeSchedule{Kind: "collection", Currency: "USD", Flat: "0.00", MarkupBps: 0}, "staff-1", svc.now()))
	require.NoError(t, svc.RecordCollection(ctx, auth.RoleVAFinance, created.ID, "USD", "100"))
	_, err = svc.SetTariff(ctx, "staff-1", auth.RoleVAFinance, "USD/UGX", "3700", 0)
	require.NoError(t, err)

	wallets, err := store.Wallets(ctx, nil, created.ID)
	require.NoError(t, err)
	var usdID, ugxID, eurID string
	for _, wallet := range wallets {
		switch wallet.Currency {
		case "USD":
			usdID = wallet.ID
		case "UGX":
			ugxID = wallet.ID
		case "EUR":
			eurID = wallet.ID
		}
	}

	_, err = svc.MoveWallets(ctx, "owner-1", auth.RoleGuide, usdID, ugxID, "10")
	require.Error(t, err)
	_, err = svc.MoveWallets(ctx, "owner-1", auth.RoleOperatorOwner, usdID, eurID, "10")
	require.Error(t, err)

	moved, err := svc.MoveWallets(ctx, "owner-1", auth.RoleOperatorOwner, usdID, ugxID, "10")
	require.NoError(t, err)
	balances := map[string]string{}
	for _, wallet := range moved.Wallets {
		balances[wallet.Currency] = wallet.Balance
	}
	require.Equal(t, "90.00", balances["USD"])
	require.Equal(t, "37000.00", balances["UGX"])

	ovaUSD, err := store.PlatformWallet(ctx, nil, "ova", "USD")
	require.NoError(t, err)
	require.Equal(t, "90.00", ovaUSD.Balance)
	ovaUGX, err := store.PlatformWallet(ctx, nil, "ova", "UGX")
	require.NoError(t, err)
	require.Equal(t, "37000.00", ovaUGX.Balance)
}
