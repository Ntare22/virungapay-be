package company

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/middleware"
)

func (s *Service) ListFeeSchedules(ctx context.Context, role string) ([]FeeSchedule, error) {
	if !staff(role) {
		return nil, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	schedules, err := s.store.ListFeeSchedules(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if schedules == nil {
		schedules = []FeeSchedule{}
	}
	return schedules, nil
}

func (s *Service) SaveFeeSchedule(ctx context.Context, actorID, role string, schedule FeeSchedule) (FeeSchedule, error) {
	if !staff(role) {
		return FeeSchedule{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	schedule.Kind = strings.ToLower(strings.TrimSpace(schedule.Kind))
	schedule.Currency = strings.ToUpper(strings.TrimSpace(schedule.Currency))
	if !validFeeKind(schedule.Kind) || !validCurrency(schedule.Currency) {
		return FeeSchedule{}, middleware.Invalid("invalid_request", "choose a transaction and USD or UGX")
	}
	if schedule.MarkupBps < 0 {
		return FeeSchedule{}, middleware.Invalid("invalid_request", "markup cannot be negative")
	}
	flat, err := money(schedule.Flat)
	if err != nil || flat.Sign() < 0 {
		return FeeSchedule{}, middleware.Invalid("invalid_request", "enter the flat fee as a number")
	}
	schedule.Flat = flat.FloatString(2)
	if err := s.store.SaveFeeSchedule(ctx, schedule, actorID, s.now()); err != nil {
		return FeeSchedule{}, middleware.Internal("internal_error", "internal server error")
	}
	return schedule, nil
}

func (s *Service) PlatformStatement(ctx context.Context, role, walletID string) (Wallet, []StatementLine, error) {
	if !staff(role) {
		return Wallet{}, nil, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	wallet, err := s.store.PlatformWalletByID(ctx, walletID)
	if errors.Is(err, errNotFound) {
		return Wallet{}, nil, middleware.NotFound("not_found", "that wallet was not found")
	}
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	movements, err := s.store.ListMovements(ctx, walletID)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	lines, err := statementLines(movements)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	return wallet, lines, nil
}

// WalletStatement is the tourist's one VirungaPass Wallet and its movements, newest first.
func (s *Service) WalletStatement(ctx context.Context, userID, role string) (Wallet, []StatementLine, error) {
	if role != auth.RoleTourist {
		return Wallet{}, nil, middleware.Forbidden("forbidden", "only a tourist wallet has this statement")
	}
	if err := s.ensureTouristWallet(ctx, userID); err != nil {
		return Wallet{}, nil, err
	}
	wallets, err := s.store.PersonalWallets(ctx, userID)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	var wallet Wallet
	for _, item := range wallets {
		if item.Kind == walletVirungaPass {
			wallet = item
			break
		}
	}
	if wallet.ID == "" {
		return Wallet{}, nil, middleware.Unprocessable("wallet_missing", "this tourist has no VirungaPass wallet")
	}
	movements, err := s.store.ListMovements(ctx, wallet.ID)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	lines, err := statementLines(movements)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	if lines == nil {
		lines = []StatementLine{}
	}
	return wallet, lines, nil
}

// Statement is one wallet the caller owns, with a running balance.
func (s *Service) Statement(ctx context.Context, userID, role, walletID string) (Wallet, []StatementLine, error) {
	wallet, err := s.store.WalletByID(ctx, walletID)
	if errors.Is(err, errNotFound) {
		return Wallet{}, nil, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	if wallet.Kind == "ova" || strings.HasPrefix(wallet.Kind, "fee_") || !validCurrency(wallet.Currency) {
		return Wallet{}, nil, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	owns, err := s.ownsWallet(ctx, userID, role, wallet)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	if !owns {
		return Wallet{}, nil, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	movements, err := s.store.ListMovements(ctx, wallet.ID)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	lines, err := statementLines(movements)
	if err != nil {
		return Wallet{}, nil, middleware.Internal("internal_error", "internal server error")
	}
	if lines == nil {
		lines = []StatementLine{}
	}
	return wallet, lines, nil
}

// CompanyStatement is each wallet of the signed-in tour operator, with a running balance.
func (s *Service) CompanyStatement(ctx context.Context, userID, role string) (string, []Wallet, [][]StatementLine, error) {
	if role != auth.RoleOperatorOwner {
		return "", nil, nil, middleware.Forbidden("forbidden", "only a tour operator can download this statement")
	}
	company, err := s.Mine(ctx, userID, role)
	if err != nil {
		return "", nil, nil, err
	}
	wallets := make([]Wallet, 0, len(company.Wallets))
	for _, wallet := range company.Wallets {
		if validCurrency(wallet.Currency) {
			wallets = append(wallets, wallet)
		}
	}
	sort.SliceStable(wallets, func(i, j int) bool {
		return currencyRank(wallets[i].Currency) < currencyRank(wallets[j].Currency)
	})
	groups := make([][]StatementLine, 0, len(wallets))
	for _, wallet := range wallets {
		movements, err := s.store.ListMovements(ctx, wallet.ID)
		if err != nil {
			return "", nil, nil, middleware.Internal("internal_error", "internal server error")
		}
		lines, err := statementLines(movements)
		if err != nil {
			return "", nil, nil, middleware.Internal("internal_error", "internal server error")
		}
		if lines == nil {
			lines = []StatementLine{}
		}
		groups = append(groups, lines)
	}
	return company.Name, wallets, groups, nil
}

func currencyRank(currency string) int {
	switch currency {
	case "UGX":
		return 0
	case "USD":
		return 1
	default:
		return 3
	}
}

func statementLines(movements []Movement) ([]StatementLine, error) {
	running := new(big.Rat)
	lines := make([]StatementLine, 0, len(movements))
	for _, movement := range movements {
		amount, err := money(movement.Amount)
		if err != nil {
			return nil, err
		}
		if movement.Direction == "out" {
			running.Sub(running, amount)
		} else {
			running.Add(running, amount)
		}
		lines = append(lines, StatementLine{
			ID: movement.ID, Kind: movement.Kind, Direction: movement.Direction,
			Amount: amount.FloatString(2), Currency: movement.Currency,
			RunningBalance: running.FloatString(2), CreatedAt: movement.CreatedAt,
		})
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines, nil
}

func (s *Service) ListPlatformWallets(ctx context.Context, role string) ([]Wallet, error) {
	if !staff(role) {
		return nil, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	wallets, err := s.store.ListPlatformWallets(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if wallets == nil {
		wallets = []Wallet{}
	}
	return wallets, nil
}

func (s *Service) RecordCollection(ctx context.Context, role, businessID, currency, amount string) error {
	if !staff(role) {
		return middleware.Forbidden("forbidden", "you cannot use this route")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !validCurrency(currency) {
		return middleware.Invalid("invalid_request", "choose USD or UGX")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount collected")
	}
	return s.post(ctx, "collection", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		business, err := s.approvedWallet(ctx, tx, businessID, currency, "operator")
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
		name := ""
		if company, err := s.store.Company(ctx, businessID); err == nil {
			name = company.Name
		}
		return s.apply(ctx, tx, "collection", currency, s.now(), gross, fee, net, txnDetail{
			OrganisationID: businessID, FromName: "Card payment", ToName: name,
		}, []posting{
			{wallet: ova.ID, delta: gross, direction: "in"},
			{wallet: business.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

func (s *Service) RecordWithdrawal(ctx context.Context, role, businessID, bureauID, currency, amount string) error {
	if !staff(role) {
		return middleware.Forbidden("forbidden", "you cannot use this route")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !validCurrency(currency) {
		return middleware.Invalid("invalid_request", "choose USD or UGX")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount to withdraw")
	}
	return s.post(ctx, "withdraw", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		operator, err := s.approvedWallet(ctx, tx, businessID, currency, "operator")
		if err != nil {
			return err
		}
		bureau, err := s.approvedWallet(ctx, tx, bureauID, currency, "bureau")
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		fromName, toName := "", ""
		if company, err := s.store.Company(ctx, businessID); err == nil {
			fromName = company.Name
		}
		if company, err := s.store.Company(ctx, bureauID); err == nil {
			toName = company.Name
		}
		return s.apply(ctx, tx, "withdraw", currency, s.now(), gross, fee, net, txnDetail{
			OrganisationID: businessID, FromName: fromName, ToName: toName,
		}, []posting{
			{wallet: operator.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: bureau.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

func (s *Service) RecordTip(ctx context.Context, role, fromUserID, toUserID, currency, amount string) error {
	if !staff(role) {
		return middleware.Forbidden("forbidden", "you cannot use this route")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	local, ok := TouristLocalCurrency("")
	if !ok || currency != local {
		return middleware.Unprocessable("unsupported_currency", "a tourist wallet is in "+local)
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the tip amount")
	}
	fromRole, err := s.store.UserRole(ctx, fromUserID)
	if errors.Is(err, errNotFound) || (err == nil && fromRole != auth.RoleTourist) {
		return middleware.Unprocessable("not_a_tourist", "tips start from a tourist wallet")
	}
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	toRole, err := s.store.UserRole(ctx, toUserID)
	if errors.Is(err, errNotFound) || (err == nil && toRole != auth.RoleGuide && toRole != auth.RoleDriver) {
		return middleware.Unprocessable("not_crew", "tips go to a guide or a driver")
	}
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return s.post(ctx, "p2p", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		from, err := s.touristWallet(ctx, tx, fromUserID, currency)
		if err != nil {
			return err
		}
		to, err := s.personalWallet(ctx, tx, toUserID, currency)
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "p2p", currency, s.now(), gross, fee, net, txnDetail{
			FromName: "Tourist", ToName: "Guide or driver",
		}, []posting{
			{wallet: from.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: to.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

// CollectPayment posts a simulated card collection for an approved operator.
// after runs in the same transaction, after the wallets move.
func (s *Service) CollectPayment(ctx context.Context, organisationID, currency, amount string, ref CardPayment, after func(tx *gorm.DB) error) error {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != "USD" {
		return middleware.Invalid("invalid_request", "card payments are in USD")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount due")
	}
	if err := s.ReadyToCollect(ctx, organisationID, currency); err != nil {
		return err
	}
	return s.post(ctx, "collection", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		business, err := s.approvedWallet(ctx, tx, organisationID, currency, "operator")
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
		name := ""
		if company, err := s.store.Company(ctx, organisationID); err == nil {
			name = company.Name
		}
		if err := s.apply(ctx, tx, "collection", currency, s.now(), gross, fee, net, txnDetail{
			OrganisationID: organisationID, FromName: ref.CustomerName, ToName: name,
			Email: ref.CustomerEmail, ItineraryID: ref.ItineraryID, InvoiceNumber: ref.InvoiceNumber,
		}, []posting{
			{wallet: ova.ID, delta: gross, direction: "in"},
			{wallet: business.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		}); err != nil {
			return err
		}
		if after == nil {
			return nil
		}
		return after(tx)
	})
}

// ReadyToCollect reports whether an approved operator can receive a card payment in this currency.
func (s *Service) ReadyToCollect(ctx context.Context, organisationID, currency string) error {
	_, err := s.approvedWallet(ctx, s.store.DB(), organisationID, currency, "operator")
	return err
}

type txnDetail struct {
	OrganisationID string
	FromName       string
	ToName         string
	Email          string
	ItineraryID    string
	InvoiceNumber  string
	QuoteCurrency  string
	QuoteAmount    string
}

type posting struct {
	wallet    string
	delta     *big.Rat
	direction string
	currency  string
}

func (s *Service) post(ctx context.Context, kind, currency string, gross *big.Rat, apply func(tx *gorm.DB, fee, net *big.Rat) error) error {
	err := s.tx(ctx, func(tx *gorm.DB) error {
		schedule, err := s.store.FeeSchedule(ctx, tx, kind, currency)
		if err != nil {
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

func (s *Service) apply(ctx context.Context, tx *gorm.DB, kind, currency string, at time.Time, gross, fee, net *big.Rat, detail txnDetail, rows []posting) error {
	groupID := uuid.NewString()
	for _, row := range rows {
		if row.delta.Sign() == 0 {
			continue
		}
		delta := row.delta.FloatString(2)
		if err := s.store.AdjustBalance(ctx, tx, row.wallet, delta, at); err != nil {
			return err
		}
		amount := delta
		if strings.HasPrefix(amount, "-") {
			amount = amount[1:]
		}
		moved := currency
		if row.currency != "" {
			moved = row.currency
		}
		if err := s.store.InsertMovement(ctx, tx, Movement{
			ID: uuid.NewString(), GroupID: groupID, Kind: kind, WalletID: row.wallet,
			Direction: row.direction, Amount: amount, Currency: moved, CreatedAt: at,
		}); err != nil {
			return err
		}
	}
	return s.store.InsertTransaction(ctx, tx, TransactionRecord{
		ID: uuid.NewString(), GroupID: groupID, Kind: kind, Currency: currency,
		Gross: gross.FloatString(2), Fee: fee.FloatString(2), Net: net.FloatString(2),
		OrganisationID: detail.OrganisationID, FromName: detail.FromName, ToName: detail.ToName,
		CounterpartyEmail: detail.Email, ItineraryID: detail.ItineraryID, InvoiceNumber: detail.InvoiceNumber,
		QuoteCurrency: detail.QuoteCurrency, QuoteAmount: detail.QuoteAmount,
		CreatedAt: at,
	})
}

// MoveWallets converts money between two wallets of one approved tour operator.
func (s *Service) MoveWallets(ctx context.Context, userID, role, fromID, toID, amount string) (Company, error) {
	if role != auth.RoleOperatorOwner {
		return Company{}, middleware.Forbidden("forbidden", "only a tour operator can move money between wallets")
	}
	if fromID == toID {
		return Company{}, middleware.Invalid("invalid_request", "choose two different wallets")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return Company{}, middleware.Invalid("invalid_request", "enter the amount to move")
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
	if company.Status != "approved" || company.Type == "forex_bureau" {
		return Company{}, middleware.Unprocessable("business_not_approved", "approve the business first")
	}
	wallets, err := s.store.Wallets(ctx, s.store.DB(), company.ID)
	if err != nil {
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	var from, to Wallet
	for _, wallet := range wallets {
		if wallet.ID == fromID {
			from = wallet
		}
		if wallet.ID == toID {
			to = wallet
		}
	}
	if from.ID == "" || to.ID == "" {
		return Company{}, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	pair, sellBase, err := exchangePair(from.Currency, to.Currency)
	if err != nil {
		return Company{}, err
	}
	rate, err := s.standingRate(ctx, pair)
	if err != nil {
		return Company{}, err
	}
	received := new(big.Rat).Quo(gross, rate)
	if sellBase {
		received = new(big.Rat).Mul(gross, rate)
	}
	receivedText := received.FloatString(2)
	rounded, ok := new(big.Rat).SetString(receivedText)
	if !ok || rounded.Sign() <= 0 {
		return Company{}, middleware.Invalid("invalid_request", "the amount is too small to convert")
	}
	err = s.tx(ctx, func(tx *gorm.DB) error {
		ovaFrom, err := s.store.PlatformWallet(ctx, tx, "ova", from.Currency)
		if err != nil {
			return err
		}
		ovaTo, err := s.store.PlatformWallet(ctx, tx, "ova", to.Currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "conversion", from.Currency, s.now(), gross, new(big.Rat), gross, txnDetail{
			OrganisationID: company.ID,
			FromName:       from.Currency + " wallet",
			ToName:         to.Currency + " wallet",
			QuoteCurrency:  to.Currency,
			QuoteAmount:    receivedText,
		}, []posting{
			{wallet: from.ID, delta: new(big.Rat).Neg(gross), direction: "out", currency: from.Currency},
			{wallet: to.ID, delta: rounded, direction: "in", currency: to.Currency},
			{wallet: ovaFrom.ID, delta: new(big.Rat).Neg(gross), direction: "out", currency: from.Currency},
			{wallet: ovaTo.ID, delta: rounded, direction: "in", currency: to.Currency},
		})
	})
	if err != nil {
		var app *middleware.AppError
		if errors.As(err, &app) {
			return Company{}, app
		}
		if errors.Is(err, errNotFound) {
			return Company{}, middleware.Unprocessable("insufficient_balance", "the wallet does not have enough")
		}
		return Company{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.withWallets(ctx, company)
}

func exchangePair(from, to string) (string, bool, error) {
	switch from + "/" + to {
	case "USD/UGX":
		return "USD/UGX", true, nil
	case "UGX/USD":
		return "USD/UGX", false, nil
	default:
		return "", false, middleware.Invalid("invalid_request", "move between UGX and USD")
	}
}

func (s *Service) standingRate(ctx context.Context, pair string) (*big.Rat, error) {
	tariffs, err := s.store.Tariffs(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	for _, tariff := range tariffs {
		if tariff.Pair != pair {
			continue
		}
		rate, ok := new(big.Rat).SetString(tariff.CustomerRate)
		if !ok || rate.Sign() <= 0 {
			break
		}
		return rate, nil
	}
	return nil, middleware.Unprocessable("rate_not_set", "no exchange rate has been set")
}

// ListTransactions returns every completed collection, withdrawal, and tip.
func (s *Service) ListTransactions(ctx context.Context, role string) ([]Transaction, error) {
	if !staff(role) {
		return nil, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	list, err := s.store.ListTransactions(ctx)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []Transaction{}
	}
	return list, nil
}

// ListOperatorPayments returns money in and out of the caller's wallets.
func (s *Service) ListOperatorPayments(ctx context.Context, userID, role string) ([]WalletActivity, error) {
	switch role {
	case auth.RoleOperatorOwner:
		member, err := s.store.Member(ctx, s.store.DB(), userID)
		if errors.Is(err, errNotFound) {
			return nil, middleware.NotFound("company_not_found", "create a company first")
		}
		if err != nil {
			return nil, middleware.Internal("internal_error", "internal server error")
		}
		list, err := s.store.ListWalletActivity(ctx, member.OrganisationID)
		if err != nil {
			return nil, middleware.Internal("internal_error", "internal server error")
		}
		if list == nil {
			list = []WalletActivity{}
		}
		return list, nil
	case auth.RoleGuide, auth.RoleDriver, auth.RoleTourist:
		list, err := s.store.ListPersonalActivity(ctx, userID)
		if err != nil {
			return nil, middleware.Internal("internal_error", "internal server error")
		}
		if list == nil {
			list = []WalletActivity{}
		}
		return list, nil
	default:
		return nil, middleware.Forbidden("forbidden", "only a tour operator, guide, driver, or tourist can view these payments")
	}
}

// RenameWallet changes the label and username of a wallet the caller owns.
// An empty name or username keeps the current value. A new username is the handle senders type.
func (s *Service) RenameWallet(ctx context.Context, userID, role, walletID, name, username string) (Wallet, error) {
	wallet, err := s.store.WalletByID(ctx, walletID)
	if errors.Is(err, errNotFound) {
		return Wallet{}, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	if err != nil {
		return Wallet{}, middleware.Internal("internal_error", "internal server error")
	}
	switch wallet.Kind {
	case "ova", "fee_fx", "fee_transaction":
		return Wallet{}, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	owns, err := s.ownsWallet(ctx, userID, role, wallet)
	if err != nil {
		return Wallet{}, middleware.Internal("internal_error", "internal server error")
	}
	if !owns {
		return Wallet{}, middleware.NotFound("wallet_not_found", "wallet was not found")
	}
	if wallet.Kind == walletVirungaPass {
		name = wallet.Name
		if strings.TrimSpace(name) == "" {
			name = virungaPassName
		}
	} else if strings.TrimSpace(name) == "" && name != "" {
		return Wallet{}, middleware.Invalid("invalid_request", "enter a wallet name up to 60 characters")
	} else {
		name = strings.TrimSpace(name)
		if name == "" {
			name = wallet.Name
		}
	}
	if name == "" || len([]rune(name)) > 60 || strings.ContainsAny(name, "\r\n\t") {
		return Wallet{}, middleware.Invalid("invalid_request", "enter a wallet name up to 60 characters")
	}
	if strings.TrimSpace(username) == "" && username != "" {
		return Wallet{}, middleware.Invalid("invalid_request", "use at least 3 letters or numbers")
	}
	if strings.TrimSpace(username) == "" {
		username = wallet.Username
	} else {
		var ok bool
		username, ok = chosenUsername(username)
		if !ok {
			return Wallet{}, middleware.Invalid("invalid_request", "use at least 3 letters or numbers")
		}
		taken, _, err := s.store.FindByUsername(ctx, username)
		if err == nil && taken.ID != wallet.ID {
			return Wallet{}, middleware.Conflict("username_taken", "that username is already used")
		}
		if err != nil && !errors.Is(err, errNotFound) {
			return Wallet{}, middleware.Internal("internal_error", "internal server error")
		}
	}
	if name == wallet.Name && username == wallet.Username {
		return wallet, nil
	}
	if err := s.store.RenameWallet(ctx, wallet.ID, name, username, s.now()); err != nil {
		if errors.Is(err, errNotFound) {
			return Wallet{}, middleware.NotFound("wallet_not_found", "wallet was not found")
		}
		return Wallet{}, middleware.Internal("internal_error", "internal server error")
	}
	wallet.Name = name
	wallet.Username = username
	return wallet, nil
}

func (s *Service) ownsWallet(ctx context.Context, userID, role string, wallet Wallet) (bool, error) {
	switch role {
	case auth.RoleOperatorOwner, auth.RoleForexBureauOwner:
		member, err := s.store.Member(ctx, s.store.DB(), userID)
		if errors.Is(err, errNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return wallet.Organisation != "" && wallet.Organisation == member.OrganisationID, nil
	case auth.RoleGuide, auth.RoleDriver, auth.RoleTourist:
		return wallet.UserID != "" && wallet.UserID == userID, nil
	default:
		return false, nil
	}
}

// MyWallets returns the personal wallets of a guide or driver, or the tourist VirungaPass Wallet.
func (s *Service) MyWallets(ctx context.Context, userID, role string) ([]Wallet, error) {
	if role != auth.RoleGuide && role != auth.RoleDriver && role != auth.RoleTourist {
		return nil, middleware.Forbidden("forbidden", "only a guide, driver, or tourist has a wallet here")
	}
	if role == auth.RoleTourist {
		if err := s.ensureTouristWallet(ctx, userID); err != nil {
			return nil, err
		}
	}
	wallets, err := s.store.PersonalWallets(ctx, userID)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if role == auth.RoleTourist {
		kept := make([]Wallet, 0, len(wallets))
		for _, wallet := range wallets {
			if wallet.Kind == walletVirungaPass {
				kept = append(kept, wallet)
			}
		}
		wallets = kept
	}
	if wallets == nil {
		wallets = []Wallet{}
	}
	return wallets, nil
}

// ApprovedBureaus lists forex bureaus that can receive a withdrawal.
func (s *Service) ApprovedBureaus(ctx context.Context, role string) (BusinessList, error) {
	switch role {
	case auth.RoleOperatorOwner, auth.RoleGuide, auth.RoleDriver, auth.RoleTourist, auth.RoleForexBureauOwner:
	default:
		return BusinessList{}, middleware.Forbidden("forbidden", "you cannot use this route")
	}
	businesses, err := s.store.ListBureaus(ctx)
	if err != nil {
		return BusinessList{}, middleware.Internal("internal_error", "internal server error")
	}
	approved := make([]Business, 0)
	for _, business := range businesses {
		if business.Status == "approved" {
			approved = append(approved, business)
		}
	}
	return BusinessList{Businesses: approved}, nil
}

// WithdrawOwn moves money from the caller's wallet to an approved forex bureau.
func (s *Service) WithdrawOwn(ctx context.Context, userID, role, bureauID, currency, amount string) error {
	switch role {
	case auth.RoleOperatorOwner, auth.RoleGuide, auth.RoleDriver:
	default:
		return middleware.Forbidden("forbidden", "only a tour operator, guide, or driver can withdraw")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !validCurrency(currency) {
		return middleware.Invalid("invalid_request", "choose USD or UGX")
	}
	gross, err := positiveMoney(amount)
	if err != nil {
		return middleware.Invalid("invalid_request", "enter the amount to withdraw")
	}
	bureau, err := s.store.Company(ctx, bureauID)
	if errors.Is(err, errNotFound) || (err == nil && (bureau.Type != "forex_bureau" || bureau.Status != "approved")) {
		return middleware.Unprocessable("not_a_bureau", "choose an approved forex bureau")
	}
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return s.post(ctx, "withdraw", currency, gross, func(tx *gorm.DB, fee, net *big.Rat) error {
		var source Wallet
		var fromName, organisationID string
		if role == auth.RoleOperatorOwner {
			member, err := s.store.Member(ctx, tx, userID)
			if errors.Is(err, errNotFound) {
				return middleware.NotFound("company_not_found", "create a company first")
			}
			if err != nil {
				return err
			}
			source, err = s.approvedWallet(ctx, tx, member.OrganisationID, currency, "operator")
			if err != nil {
				return err
			}
			organisationID = member.OrganisationID
			if company, err := s.store.Company(ctx, member.OrganisationID); err == nil {
				fromName = company.Name
			}
		} else {
			source, err = s.personalWallet(ctx, tx, userID, currency)
			if err != nil {
				return err
			}
			fromName = "Personal wallet"
		}
		destination, err := s.approvedWallet(ctx, tx, bureauID, currency, "bureau")
		if err != nil {
			return err
		}
		fees, err := s.store.PlatformWallet(ctx, tx, "fee_transaction", currency)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, "withdraw", currency, s.now(), gross, fee, net, txnDetail{
			OrganisationID: organisationID, FromName: fromName, ToName: bureau.Name,
		}, []posting{
			{wallet: source.ID, delta: new(big.Rat).Neg(gross), direction: "out"},
			{wallet: destination.ID, delta: net, direction: "in"},
			{wallet: fees.ID, delta: fee, direction: "in"},
		})
	})
}

func (s *Service) approvedWallet(ctx context.Context, tx *gorm.DB, organisationID, currency, kind string) (Wallet, error) {
	company, err := s.store.Company(ctx, organisationID)
	if errors.Is(err, errNotFound) {
		return Wallet{}, middleware.NotFound("business_not_found", "business was not found")
	}
	if err != nil {
		return Wallet{}, err
	}
	if company.Status != "approved" {
		return Wallet{}, middleware.Unprocessable("business_not_approved", "approve the business first")
	}
	if kind == "bureau" && company.Type != "forex_bureau" {
		return Wallet{}, middleware.Unprocessable("not_a_bureau", "choose a forex bureau")
	}
	if kind == "operator" && company.Type == "forex_bureau" {
		return Wallet{}, middleware.Unprocessable("not_an_operator", "choose a tour operator")
	}
	wallets, err := s.store.Wallets(ctx, tx, organisationID)
	if err != nil {
		return Wallet{}, err
	}
	for _, wallet := range wallets {
		if wallet.Currency == currency {
			return wallet, nil
		}
	}
	return Wallet{}, middleware.Unprocessable("wallet_missing", "open a "+currency+" wallet first")
}

func (s *Service) ensureTouristWallet(ctx context.Context, userID string) error {
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
		ID: uuid.NewString(), Kind: walletVirungaPass, Name: virungaPassName,
		Currency: currency, Balance: "0.00", Default: true, UserID: userID,
	}
	if err := s.store.InsertWallet(ctx, s.store.DB(), created, s.now()); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}

func (s *Service) touristWallet(ctx context.Context, tx *gorm.DB, userID, currency string) (Wallet, error) {
	wallet, err := s.store.PersonalWallet(ctx, tx, userID, currency)
	if errors.Is(err, errNotFound) || (err == nil && wallet.Kind != walletVirungaPass) {
		return Wallet{}, middleware.Unprocessable("wallet_missing", "this tourist has no VirungaPass wallet")
	}
	if err != nil {
		return Wallet{}, err
	}
	return wallet, nil
}

func (s *Service) personalWallet(ctx context.Context, tx *gorm.DB, userID, currency string) (Wallet, error) {
	wallet, err := s.store.PersonalWallet(ctx, tx, userID, currency)
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, errNotFound) {
		return Wallet{}, err
	}
	created := Wallet{ID: uuid.NewString(), Kind: "personal", Currency: currency, Balance: "0.00", UserID: userID}
	if err := s.store.InsertWallet(ctx, tx, created, s.now()); err != nil {
		return Wallet{}, err
	}
	return created, nil
}

func charge(gross *big.Rat, flat string, bps int) (*big.Rat, error) {
	flatMoney, err := money(flat)
	if err != nil {
		return nil, err
	}
	percent := new(big.Rat).Mul(gross, big.NewRat(int64(bps), 10000))
	return new(big.Rat).Add(flatMoney, percent), nil
}

func positiveMoney(value string) (*big.Rat, error) {
	amount, err := money(value)
	if err != nil || amount.Sign() <= 0 {
		return nil, errors.New("invalid amount")
	}
	return amount, nil
}

func money(value string) (*big.Rat, error) {
	amount, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return nil, errors.New("invalid amount")
	}
	return amount, nil
}

func validCurrency(currency string) bool {
	return currency == "USD" || currency == "UGX"
}

func validFeeKind(kind string) bool {
	return kind == "collection" || kind == "withdraw" || kind == "p2p"
}
