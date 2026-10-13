package company

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	walletVirungaPass = "virungapass"
	virungaPassName   = "VirungaPass Wallet"
)

// eastAfricaCurrency is the local wallet currency for an open East African market.
// Uganda is the only open market. Add the next country here, and allow that currency
// on the wallets table, when that market opens.
var eastAfricaCurrency = map[string]string{
	"UG": "UGX",
}

// TouristLocalCurrency returns the wallet currency for an East African country.
// An empty country uses Uganda, the only open market.
func TouristLocalCurrency(country string) (string, bool) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "" {
		country = "UG"
	}
	currency, ok := eastAfricaCurrency[country]
	return currency, ok
}

// OpenTouristWallet creates the tourist's one VirungaPass Wallet in the local currency.
func (r *Repository) OpenTouristWallet(ctx context.Context, userID string) error {
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return errNotFound
	}
	if err := r.InsertWallet(ctx, r.db, Wallet{
		ID: uuid.NewString(), Kind: walletVirungaPass, Name: virungaPassName,
		Currency: currency, Balance: "0.00", Default: true, UserID: userID,
	}, time.Now().UTC()); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(`
UPDATE itineraries
SET tourist_user_id = ?, updated_at = NOW()
WHERE status = 'paid' AND tourist_user_id IS NULL
  AND lower(customer_email) = (SELECT lower(email) FROM users WHERE id = ?)
`, userID, userID).Error
}
