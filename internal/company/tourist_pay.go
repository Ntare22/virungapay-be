package company

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/middleware"
)

// TopUpCard adds UGX to the signed-in tourist's VirungaPass Wallet from a bank card.
// last4 is the only card detail kept, so the payment can be recognised later.
func (s *Service) TopUpCard(ctx context.Context, userID, role, amount, last4 string) error {
	if role != auth.RoleTourist {
		return middleware.Forbidden("forbidden", "only a tourist can add money to a VirungaPass wallet")
	}
	if len(last4) != 4 || strings.Trim(last4, "0123456789") != "" {
		return middleware.Invalid("invalid_request", "enter the card number")
	}
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return middleware.Internal("internal_error", "internal server error")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount to add")
	}
	if err := s.ensureTouristWallet(ctx, userID); err != nil {
		return err
	}
	return s.postLoose(ctx, "collection", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		wallet, err := s.touristWallet(ctx, tx, userID, currency)
		if err != nil {
			return err
		}
		ova, err := s.store.PlatformWallet(ctx, tx, "ova", currency)
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "collection", currency, s.now(), gross, fee, net, txnDetail{
			FromName: "Card •••• " + last4, ToName: virungaPassName,
		}, []posting{
			{wallet: ova.ID, delta: gross, direction: "in"},
			{wallet: wallet.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

// SendMobileMoney pays an Airtel or MTN Uganda number from the tourist wallet.
func (s *Service) SendMobileMoney(ctx context.Context, userID, role, network, phone, amount string) error {
	if role != auth.RoleTourist {
		return middleware.Forbidden("forbidden", "only a tourist can send to mobile money")
	}
	label, ok := ugandaNetwork(network)
	if !ok {
		return middleware.Invalid("invalid_request", "choose Airtel or MTN")
	}
	phone, ok = ugandaMobile(phone)
	if !ok {
		return middleware.Invalid("invalid_request", "enter a Uganda mobile number")
	}
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return middleware.Internal("internal_error", "internal server error")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount to send")
	}
	return s.postLoose(ctx, "withdraw", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		wallet, err := s.touristWallet(ctx, tx, userID, currency)
		if err != nil {
			return err
		}
		ova, err := s.store.PlatformWallet(ctx, tx, "ova", currency)
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "withdraw", currency, s.now(), gross, fee, net, txnDetail{
			FromName: virungaPassName, ToName: label + " " + phone,
		}, []posting{
			{wallet: wallet.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: ova.ID, delta: new(big.Rat).Neg(net), direction: "out"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

// TipCrew moves money from the signed-in tourist to a wallet username.
// purpose is "tip" for a guide or driver, or "service" to pay a tour operator.
func (s *Service) TipCrew(ctx context.Context, userID, role, username, amount, purpose string) error {
	if role != auth.RoleTourist {
		return middleware.Forbidden("forbidden", "only a tourist can send from a VirungaPass wallet")
	}
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if purpose == "" {
		purpose = "tip"
	}
	if purpose != "tip" && purpose != "service" {
		return middleware.Invalid("invalid_request", "choose a tip or a payment for a service")
	}
	username = normalizeUsername(username)
	if username == "" {
		return middleware.Invalid("invalid_request", "enter a wallet username")
	}
	currency, ok := TouristLocalCurrency("")
	if !ok {
		return middleware.Internal("internal_error", "internal server error")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		if purpose == "service" {
			return middleware.Invalid("invalid_request", "enter the amount to pay")
		}
		return middleware.Invalid("invalid_request", "enter the tip amount")
	}
	wallet, owner, err := s.store.FindByUsername(ctx, username)
	if errors.Is(err, errNotFound) || wallet.ID == "" {
		return middleware.NotFound("wallet_not_found", "no wallet uses that username")
	}
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	if wallet.Currency != currency {
		return middleware.Unprocessable("unsupported_currency", "this payment uses the "+currency+" wallet")
	}
	if purpose == "service" {
		return s.payOperator(ctx, userID, currency, gross, wallet, owner)
	}
	if wallet.UserID == "" || wallet.Kind != "personal" {
		return middleware.Unprocessable("not_crew", "tips go to a guide or a driver")
	}
	toRole, err := s.store.UserRole(ctx, wallet.UserID)
	if errors.Is(err, errNotFound) || (err == nil && toRole != auth.RoleGuide && toRole != auth.RoleDriver) {
		return middleware.Unprocessable("not_crew", "tips go to a guide or a driver")
	}
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	if owner == "" {
		owner = "Guide or driver"
	}
	return s.postLoose(ctx, "p2p", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		from, err := s.touristWallet(ctx, tx, userID, currency)
		if err != nil {
			return err
		}
		to, err := s.personalWallet(ctx, tx, wallet.UserID, currency)
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "p2p", currency, s.now(), gross, fee, net, txnDetail{
			FromName: "Tourist", ToName: owner,
		}, []posting{
			{wallet: from.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: to.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

func (s *Service) payOperator(ctx context.Context, userID, currency string, gross *big.Rat, wallet Wallet, owner string) error {
	if wallet.Kind != "operator" || wallet.Organisation == "" {
		return middleware.Unprocessable("not_operator", "payments for a service go to a tour operator")
	}
	if owner == "" {
		owner = wallet.Name
	}
	if owner == "" {
		owner = "Tour operator"
	}
	return s.postLoose(ctx, "p2p", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		from, err := s.touristWallet(ctx, tx, userID, currency)
		if err != nil {
			return err
		}
		to, err := s.approvedWallet(ctx, tx, wallet.Organisation, currency, "operator")
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "p2p", currency, s.now(), gross, fee, net, txnDetail{
			OrganisationID: wallet.Organisation, FromName: "Tourist", ToName: owner, InvoiceNumber: "service",
		}, []posting{
			{wallet: from.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: to.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

func (s *Service) postLoose(ctx context.Context, kind, currency string, gross *big.Rat, apply func(tx *gorm.DB, fee, net *big.Rat) error) error {
	err := s.tx(ctx, func(tx *gorm.DB) error {
		schedule, err := s.store.FeeSchedule(ctx, tx, kind, currency)
		if errors.Is(err, errNotFound) {
			schedule = FeeSchedule{Flat: "0.00"}
		} else if err != nil {
			return err
		}
		fee, err := charge(gross, schedule.Flat, schedule.MarkupBps)
		if err != nil {
			return err
		}
		if fee.Cmp(gross) > 0 {
			return middleware.Unprocessable("fee_too_large", "the fee is larger than the amount")
		}
		net := new(big.Rat).Sub(gross, fee)
		return apply(tx, fee, net)
	})
	if err == nil {
		return nil
	}
	var app *middleware.AppError
	if errors.As(err, &app) {
		return app
	}
	if errors.Is(err, errNotFound) {
		return middleware.Unprocessable("insufficient_balance", "the wallet does not have enough")
	}
	return middleware.Internal("internal_error", "internal server error")
}

func ugandaNetwork(network string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "airtel":
		return "Airtel", true
	case "mtn":
		return "MTN", true
	default:
		return "", false
	}
}

func ugandaMobile(phone string) (string, bool) {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	number := digits.String()
	switch {
	case strings.HasPrefix(number, "256") && len(number) == 12:
		number = number[3:]
	case strings.HasPrefix(number, "0") && len(number) == 10:
		number = number[1:]
	}
	if len(number) != 9 || number[0] != '7' {
		return "", false
	}
	return "+256" + number, true
}
