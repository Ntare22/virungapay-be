// Package company lets an operator owner create one tour business and invite its crew.
package company

import "time"

// Organisation is the organisations table.
type Organisation struct {
	ID                  string    `gorm:"type:uuid;primaryKey"`
	Type                string    `gorm:"type:organisation_type;not null"`
	Name                string    `gorm:"not null"`
	Country             string    `gorm:"not null"`
	AssociationMemberOf *string   `gorm:"type:association_code"`
	Status              string    `gorm:"type:organisation_status;not null"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

func (Organisation) TableName() string { return "organisations" }

// CreateInput is a new tour company.
type CreateInput struct {
	Name        string
	Country     string
	Association string
}

// InviteInput is a guide or driver invitation.
type InviteInput struct {
	Name  string
	Email string
	Phone string
	Role  string
}

// Company is the operator organisation the caller owns.
type Company struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Country             string   `json:"country"`
	AssociationMemberOf *string  `json:"association_member_of,omitempty"`
	Status              string   `json:"status"`
	Type                string   `json:"type,omitempty"`
	Wallets             []Wallet `json:"wallets"`
}

// PersonWallet is a tourist, guide, or driver wallet, with the person who holds it.
type PersonWallet struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Username     string `json:"username,omitempty"`
	Currency     string `json:"currency"`
	Balance      string `json:"balance"`
	HolderName   string `json:"holder_name"`
	HolderEmail  string `json:"holder_email"`
	HolderRole   string `json:"holder_role"`
	HolderStatus string `json:"holder_status"`
}

// TouristAccount is a tourist an operator can attach an itinerary to.
type TouristAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
}

// Wallet is one currency account. Balance is the claim on money in that currency.
type Wallet struct {
	ID           string `json:"id"`
	Kind         string `json:"kind,omitempty"`
	Name         string `json:"name,omitempty"`
	Username     string `json:"username,omitempty"`
	Currency     string `json:"currency"`
	Balance      string `json:"balance"`
	Default      bool   `json:"is_default"`
	Organisation string `json:"-"`
	UserID       string `json:"-"`
	CreatedBy    string `json:"-"`
}

// Business is a company plus its owner, for the staff list.
type Business struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Country             string   `json:"country"`
	AssociationMemberOf *string  `json:"association_member_of,omitempty"`
	Status              string   `json:"status"`
	OwnerName           string   `json:"owner_name"`
	OwnerEmail          string   `json:"owner_email"`
	Wallets             []Wallet `json:"wallets"`
}

// BusinessList is the staff list of operator companies.
type BusinessList struct {
	Businesses []Business `json:"businesses"`
}

// Tariff is the customer rate for one currency pair on one calendar day.
type Tariff struct {
	ID             string `json:"-"`
	Pair           string `json:"pair"`
	QuoteOn        string `json:"quote_on"`
	IntegratorRate string `json:"integrator_rate"`
	MarkupBps      int    `json:"markup_bps"`
	CustomerRate   string `json:"customer_rate"`
}

// TariffRequest is the day's quotation and VirungaPay's markup.
type TariffRequest struct {
	Pair           string `json:"pair" binding:"required" example:"USD/UGX"`
	IntegratorRate string `json:"integrator_rate" binding:"required"`
	MarkupBps      int    `json:"markup_bps" binding:"min=0"`
}

// FeeSchedule is the flat amount and basis-point fee for one transaction kind and currency.
type FeeSchedule struct {
	Kind      string `json:"kind"`
	Currency  string `json:"currency"`
	Flat      string `json:"flat"`
	MarkupBps int    `json:"markup_bps"`
}

// BureauRequest is a forex bureau added by staff.
type BureauRequest struct {
	Name    string `json:"name" binding:"required"`
	Country string `json:"country" binding:"required"`
	Owner   string `json:"owner" binding:"required"`
	Email   string `json:"email" binding:"required"`
	Phone   string `json:"phone"`
}
type MoveRequest struct {
	FromWalletID string `json:"from_wallet_id" binding:"required"`
	ToWalletID   string `json:"to_wallet_id" binding:"required"`
	Amount       string `json:"amount" binding:"required"`
}
type FeeScheduleRequest struct {
	Kind      string `json:"kind" binding:"required"`
	Currency  string `json:"currency" binding:"required"`
	Flat      string `json:"flat" binding:"required"`
	MarkupBps int    `json:"markup_bps" binding:"min=0"`
}

// Movement is one side of a grouped wallet posting.
type Movement struct {
	ID        string
	GroupID   string
	Kind      string
	WalletID  string
	Direction string
	Amount    string
	Currency  string
	CreatedAt time.Time
}

// StatementLine is one movement on a wallet statement, with the balance after it.
type StatementLine struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	Direction      string    `json:"direction"`
	Amount         string    `json:"amount"`
	Currency       string    `json:"currency"`
	RunningBalance string    `json:"running_balance"`
	CreatedAt      time.Time `json:"created_at"`
}

// CardPayment is the invoice a customer pays by card.
type CardPayment struct {
	ItineraryID   string
	InvoiceNumber string
	CustomerName  string
	CustomerEmail string
}

// TransactionRecord is written with the wallet movements for one payment.
type TransactionRecord struct {
	ID                string
	GroupID           string
	Kind              string
	Currency          string
	Gross             string
	Fee               string
	Net               string
	OrganisationID    string
	FromName          string
	ToName            string
	CounterpartyEmail string
	ItineraryID       string
	InvoiceNumber     string
	QuoteCurrency     string
	QuoteAmount       string
	CreatedAt         time.Time
}
type Transaction struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Currency      string    `json:"currency"`
	Gross         string    `json:"gross"`
	Fee           string    `json:"fee"`
	Net           string    `json:"net"`
	FromName      string    `json:"from_name"`
	ToName        string    `json:"to_name"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	QuoteCurrency string    `json:"quote_currency,omitempty"`
	QuoteAmount   string    `json:"quote_amount,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// WalletActivity is money in or out of one business wallet.
type WalletActivity struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Direction     string    `json:"direction"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	Counterparty  string    `json:"counterparty,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CollectionRequest records a card payment collected by an operator.
type CollectionRequest struct {
	BusinessID string `json:"business_id" binding:"required"`
	Currency   string `json:"currency" binding:"required"`
	Amount     string `json:"amount" binding:"required"`
}

// WithdrawalRequest moves money from an operator wallet to a forex bureau.
type WithdrawalRequest struct {
	BusinessID string `json:"business_id" binding:"required"`
	BureauID   string `json:"bureau_id" binding:"required"`
	Currency   string `json:"currency" binding:"required"`
	Amount     string `json:"amount" binding:"required"`
}

// OwnWithdrawalRequest moves money from the signed-in person's wallet to a forex bureau.
type OwnWithdrawalRequest struct {
	BureauID string `json:"bureau_id" binding:"required"`
	Currency string `json:"currency" binding:"required"`
	Amount   string `json:"amount" binding:"required"`
}

// TipRequest moves money from a tourist wallet to a guide or driver.
type TipRequest struct {
	FromUserID string `json:"from_user_id" binding:"required"`
	ToUserID   string `json:"to_user_id" binding:"required"`
	Currency   string `json:"currency" binding:"required"`
	Amount     string `json:"amount" binding:"required"`
}
type WalletRequest struct {
	Currency string `json:"currency" binding:"required" example:"USD"`
}

// TopUpRequest adds money to the tourist VirungaPass Wallet from a bank card.
// Only the last four digits are sent. The full card number stays on the phone.
type TopUpRequest struct {
	Amount string `json:"amount" binding:"required"`
	Last4  string `json:"last4" binding:"required" example:"4242"`
}

// MobileMoneyRequest sends UGX from the tourist wallet to Airtel or MTN in Uganda.
type MobileMoneyRequest struct {
	Network string `json:"network" binding:"required" example:"mtn"`
	Phone   string `json:"phone" binding:"required" example:"+256771234567"`
	Amount  string `json:"amount" binding:"required"`
}

// OwnTipRequest sends money from the signed-in tourist to a wallet username.
// Purpose is tip for a guide or driver, or service for a tour operator.
type OwnTipRequest struct {
	Username string `json:"username" binding:"required" example:"peter-ugx"`
	Amount   string `json:"amount" binding:"required"`
	Purpose  string `json:"purpose" example:"tip"`
}

// RenameWalletRequest updates the label and the username senders use to find a wallet.
type RenameWalletRequest struct {
	Name     string `json:"name" example:"Safari float"`
	Username string `json:"username" example:"safari-float"`
}

// CreateRequest is the JSON body for creating a company.
type CreateRequest struct {
	Name        string `json:"name" binding:"required" example:"Kigezi Highland Safaris"`
	Country     string `json:"country" binding:"required,len=2" example:"UG"`
	Association string `json:"association" example:"AUTO"`
}

// InviteRequest is the JSON body for inviting a guide or driver.
type InviteRequest struct {
	Name  string `json:"name" binding:"required" example:"Peter Mugisha"`
	Email string `json:"email" binding:"required,email" example:"peter@example.com"`
	Phone string `json:"phone" example:"+256700000001"`
	Role  string `json:"role" binding:"required" example:"driver"`
}

// CrewMember is a guide or driver on the operator's company, with their duty wallet.
type CrewMember struct {
	UserID         string `json:"user_id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone,omitempty"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	WalletCurrency string `json:"wallet_currency"`
	WalletBalance  string `json:"wallet_balance"`
	Username       string `json:"username,omitempty"`
}

// WalletLookup is what a sender sees before money moves to a wallet.
type WalletLookup struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Kind     string `json:"kind"`
}

// InviteResponse acknowledges an emailed invite. It does not include the token.
type InviteResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// Member links one person to one company.
type Member struct {
	OrganisationID string
	UserID         string
	Role           string
	CreatedAt      time.Time
}

// CrewList is the guides and drivers on the signed-in operator's company.
type CrewList struct {
	Crew []CrewMember `json:"crew"`
}

// PersonWalletList is tourist, guide, and driver wallets for staff.
type PersonWalletList struct {
	Wallets []PersonWallet `json:"wallets"`
}

// TouristList is tourist accounts an operator can attach to an itinerary.
type TouristList struct {
	Tourists []TouristAccount `json:"tourists"`
}

// TariffList is the day's customer rates.
type TariffList struct {
	Tariffs []Tariff `json:"tariffs"`
}

// FeeList is the fee schedule for each kind and currency.
type FeeList struct {
	Fees []FeeSchedule `json:"fees"`
}

// WalletList is a set of wallets.
type WalletList struct {
	Wallets []Wallet `json:"wallets"`
}

// StatementView is one wallet and the movements that produced its balance.
type StatementView struct {
	Wallet    Wallet          `json:"wallet"`
	Movements []StatementLine `json:"movements"`
}

// TransactionList is staff's view of recorded payments.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
}

// PaymentList is money in and out of the signed-in operator's wallets.
type PaymentList struct {
	Payments []WalletActivity `json:"payments"`
}

// Recorded acknowledges a simulated money movement.
type Recorded struct {
	Status string `json:"status" example:"recorded"`
}

// Invite is a stored company invite. Only the token hash is persisted.
type Invite struct {
	ID             string
	OrganisationID string
	UserID         string
	Role           string
	TokenHash      string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	CreatedAt      time.Time
}
