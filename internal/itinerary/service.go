package itinerary

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/company"
	appmail "virungapay-be/internal/mail"
	"virungapay-be/internal/middleware"
)

// Tx runs a function inside one database transaction.
type Tx func(ctx context.Context, fn func(tx *gorm.DB) error) error

// Payer records a simulated card collection on the operator wallet.
type Payer interface {
	ReadyToCollect(ctx context.Context, organisationID, currency string) error
	CollectPayment(ctx context.Context, organisationID, currency, amount string, ref company.CardPayment, after func(tx *gorm.DB) error) error
}

// Store is itinerary persistence.
type Store interface {
	Operator(ctx context.Context, userID string) (Operator, error)
	ListPackages(ctx context.Context, organisationID string) ([]Package, error)
	Package(ctx context.Context, db *gorm.DB, organisationID, id string) (Package, error)
	SavePackage(ctx context.Context, db *gorm.DB, pkg Package, at time.Time) error
	ListInquiries(ctx context.Context, organisationID string) ([]Inquiry, error)
	Inquiry(ctx context.Context, organisationID, id string) (Inquiry, error)
	InsertInquiry(ctx context.Context, inquiry Inquiry) error
	ListItineraries(ctx context.Context, organisationID string) ([]Itinerary, error)
	Itinerary(ctx context.Context, organisationID, id string) (Itinerary, error)
	ItineraryByToken(ctx context.Context, hash string) (Itinerary, error)
	SaveItinerary(ctx context.Context, db *gorm.DB, item Itinerary, at time.Time) error
	NextInvoice(ctx context.Context, db *gorm.DB, organisationID string, year int) (string, error)
	MarkSent(ctx context.Context, db *gorm.DB, id, invoice, tokenHash, currency, amount string, at time.Time) error
	RevertSent(ctx context.Context, id string, at time.Time) error
	MarkPaid(ctx context.Context, db *gorm.DB, id string, at time.Time) error
	UserEmail(ctx context.Context, userID string) (string, error)
	ListMine(ctx context.Context, userID, email string) ([]Itinerary, error)
	TouristID(ctx context.Context, userID string) (string, error)
	TouristIDByEmail(ctx context.Context, email string) (string, error)
	AssignTourist(ctx context.Context, itineraryID, touristID string, at time.Time) error
	DB() *gorm.DB
}

// Service lets a tour operator curate packages and send priced itineraries.
type Service struct {
	store  Store
	payer  Payer
	mail   appmail.Sender
	tx     Tx
	public string
	pwa    string
	now    func() time.Time
}

// NewService builds the itinerary service. public is the website origin used in pay links. pwa is the phone origin used in signup links.
func NewService(store Store, payer Payer, sender appmail.Sender, run Tx, publicURL, pwaURL string) *Service {
	return &Service{
		store: store, payer: payer, mail: sender, tx: run,
		public: strings.TrimRight(publicURL, "/"),
		pwa:    strings.TrimRight(pwaURL, "/"),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) ListPackages(ctx context.Context, userID, role string) ([]Package, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListPackages(ctx, op.OrganisationID)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []Package{}
	}
	return list, nil
}

func (s *Service) SavePackage(ctx context.Context, userID, role string, pkg Package) (Package, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Package{}, err
	}
	pkg.Title = strings.TrimSpace(pkg.Title)
	pkg.Summary = strings.TrimSpace(pkg.Summary)
	if pkg.Title == "" {
		return Package{}, middleware.Invalid("invalid_request", "give the package a title")
	}
	if pkg.Status != "published" {
		pkg.Status = "draft"
	}
	pkg.Days = cleanDays(pkg.Days)
	pkg.Inclusions = cleanLines(pkg.Inclusions)
	pkg.Exclusions = cleanLines(pkg.Exclusions)
	pkg.Notes = cleanLines(pkg.Notes)
	pkg.Organisation = op.OrganisationID
	if pkg.ID == "" {
		pkg.ID = uuid.NewString()
	}
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		return s.store.SavePackage(ctx, tx, pkg, s.now())
	}); err != nil {
		return Package{}, middleware.Internal("internal_error", "internal server error")
	}
	return s.store.Package(ctx, s.store.DB(), op.OrganisationID, pkg.ID)
}

func (s *Service) ListInquiries(ctx context.Context, userID, role string) ([]Inquiry, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListInquiries(ctx, op.OrganisationID)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []Inquiry{}
	}
	return list, nil
}

func (s *Service) CreateInquiry(ctx context.Context, userID, role string, inquiry Inquiry) (Inquiry, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Inquiry{}, err
	}
	normalized, err := normalizeInquiry(inquiry)
	if err != nil {
		return Inquiry{}, err
	}
	normalized.ID = uuid.NewString()
	normalized.Organisation = op.OrganisationID
	normalized.CreatedAt = s.now()
	if err := s.store.InsertInquiry(ctx, normalized); err != nil {
		return Inquiry{}, middleware.Internal("internal_error", "internal server error")
	}
	return normalized, nil
}

func (s *Service) ListItineraries(ctx context.Context, userID, role string) ([]Itinerary, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListItineraries(ctx, op.OrganisationID)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []Itinerary{}
	}
	return list, nil
}

func (s *Service) Mine(ctx context.Context, userID, role string) ([]Itinerary, error) {
	if role != auth.RoleTourist {
		return nil, middleware.Forbidden("forbidden", "only a tourist can view these itineraries")
	}
	email, err := s.store.UserEmail(ctx, userID)
	if errors.Is(err, errNotFound) {
		return nil, middleware.NotFound("not_found", "sign in again")
	}
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	list, err := s.store.ListMine(ctx, userID, email)
	if err != nil {
		return nil, middleware.Internal("internal_error", "internal server error")
	}
	if list == nil {
		list = []Itinerary{}
	}
	return list, nil
}

func (s *Service) GetItinerary(ctx context.Context, userID, role, id string) (Itinerary, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Itinerary{}, err
	}
	item, err := s.store.Itinerary(ctx, op.OrganisationID, id)
	if errors.Is(err, errNotFound) {
		return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
	}
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	item.BusinessName = op.Name
	return item, nil
}

func (s *Service) SaveItinerary(ctx context.Context, userID, role string, item Itinerary) (Itinerary, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Itinerary{}, err
	}
	if item.PackageID != "" {
		pkg, err := s.store.Package(ctx, s.store.DB(), op.OrganisationID, item.PackageID)
		if errors.Is(err, errNotFound) {
			return Itinerary{}, middleware.NotFound("not_found", "that package was not found")
		}
		if err != nil {
			return Itinerary{}, middleware.Internal("internal_error", "internal server error")
		}
		if item.Title == "" {
			item.Title = pkg.Title
		}
		if item.Summary == "" {
			item.Summary = pkg.Summary
		}
		if len(item.Days) == 0 {
			item.Days = pkg.Days
		}
		if len(item.Inclusions) == 0 {
			item.Inclusions = pkg.Inclusions
		}
		if len(item.Exclusions) == 0 {
			item.Exclusions = pkg.Exclusions
		}
		if len(item.Notes) == 0 {
			item.Notes = pkg.Notes
		}
	}
	if item.InquiryID != "" {
		inquiry, err := s.store.Inquiry(ctx, op.OrganisationID, item.InquiryID)
		if errors.Is(err, errNotFound) {
			return Itinerary{}, middleware.NotFound("not_found", "that inquiry was not found")
		}
		if err != nil {
			return Itinerary{}, middleware.Internal("internal_error", "internal server error")
		}
		if item.CustomerName == "" {
			item.CustomerName = inquiry.CustomerName
		}
		if item.CustomerEmail == "" {
			item.CustomerEmail = inquiry.CustomerEmail
		}
		if item.StartsOn == "" {
			item.StartsOn = inquiry.StartsOn
		}
		if item.EndsOn == "" {
			item.EndsOn = inquiry.EndsOn
		}
		if item.PartySize == 0 {
			item.PartySize = inquiry.PartySize
		}
		if item.Summary == "" {
			item.Summary = inquiry.Notes
		}
	}
	normalized, err := normalizeItinerary(item)
	if err != nil {
		return Itinerary{}, err
	}
	normalized.Organisation = op.OrganisationID
	normalized.Status = "draft"
	if normalized.ID == "" {
		normalized.ID = uuid.NewString()
	} else {
		current, err := s.store.Itinerary(ctx, op.OrganisationID, normalized.ID)
		if errors.Is(err, errNotFound) {
			return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
		}
		if err != nil {
			return Itinerary{}, middleware.Internal("internal_error", "internal server error")
		}
		if current.Status != "draft" {
			return Itinerary{}, middleware.Unprocessable("already_sent", "a sent itinerary cannot be edited")
		}
	}
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		return s.store.SaveItinerary(ctx, tx, normalized, s.now())
	}); err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	saved, err := s.store.Itinerary(ctx, op.OrganisationID, normalized.ID)
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	saved.BusinessName = op.Name
	return saved, nil
}

func (s *Service) Send(ctx context.Context, userID, role, id string) (Itinerary, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Itinerary{}, err
	}
	item, err := s.store.Itinerary(ctx, op.OrganisationID, id)
	if errors.Is(err, errNotFound) {
		return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
	}
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	if item.Status != "draft" {
		return Itinerary{}, middleware.Unprocessable("already_sent", "this itinerary was already sent")
	}
	if item.Currency != "USD" && item.Currency != "EUR" {
		return Itinerary{}, middleware.Invalid("invalid_request", "choose USD or EUR")
	}
	if item.AmountDue == "" || item.AmountDue == "0.00" || item.AmountDue == "0" {
		return Itinerary{}, middleware.Invalid("invalid_request", "enter the amount due before sending")
	}
	if err := s.payer.ReadyToCollect(ctx, op.OrganisationID, item.Currency); err != nil {
		return Itinerary{}, err
	}
	raw, hash, err := newToken()
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	var invoice string
	now := s.now()
	if err := s.tx(ctx, func(tx *gorm.DB) error {
		number, err := s.store.NextInvoice(ctx, tx, op.OrganisationID, now.Year())
		if err != nil {
			return err
		}
		invoice = number
		return s.store.MarkSent(ctx, tx, item.ID, invoice, hash, item.Currency, item.AmountDue, now)
	}); err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	item.InvoiceNumber = invoice
	item.Status = "sent"
	item.BusinessName = op.Name
	payURL := s.public + "/pay/" + raw
	signupURL := s.pwa + "/signup?email=" + url.QueryEscape(item.CustomerEmail)
	html, err := appmail.RenderItinerary(mailData(item, payURL, signupURL))
	if err != nil {
		_ = s.store.RevertSent(ctx, item.ID, s.now())
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	text := fmt.Sprintf("%s sent itinerary %s.\nInvoice %s\n%s %s\nPay: %s\nCreate a VirungaPay account with this same email so this trip appears on Home: %s\n", op.Name, item.Title, invoice, item.Currency, item.AmountDue, payURL, signupURL)
	if err := s.mail.Send(ctx, appmail.Message{
		ToEmail: item.CustomerEmail, ToName: item.CustomerName,
		Subject: "Itinerary " + invoice + " from " + op.Name,
		Text:    text, HTML: html,
	}); err != nil {
		_ = s.store.RevertSent(ctx, item.ID, s.now())
		return Itinerary{}, middleware.Unavailable("mail_unavailable", "email could not be sent")
	}
	sent, err := s.store.Itinerary(ctx, op.OrganisationID, item.ID)
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	sent.BusinessName = op.Name
	return sent, nil
}

func (s *Service) PublicItinerary(ctx context.Context, token string) (Itinerary, error) {
	item, err := s.store.ItineraryByToken(ctx, hashToken(token))
	if errors.Is(err, errNotFound) {
		return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
	}
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	return item, nil
}

func (s *Service) Pay(ctx context.Context, token string) (Itinerary, error) {
	item, err := s.PublicItinerary(ctx, token)
	if err != nil {
		return Itinerary{}, err
	}
	if item.Status == "paid" {
		return Itinerary{}, middleware.Unprocessable("already_paid", "this itinerary is already paid")
	}
	if item.Status != "sent" {
		return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
	}
	if err := s.payer.CollectPayment(ctx, item.Organisation, item.Currency, item.AmountDue, company.CardPayment{
		ItineraryID: item.ID, InvoiceNumber: item.InvoiceNumber, CustomerName: item.CustomerName, CustomerEmail: item.CustomerEmail,
	}, func(tx *gorm.DB) error {
		return s.store.MarkPaid(ctx, tx, item.ID, s.now())
	}); err != nil {
		return Itinerary{}, err
	}
	paid, err := s.store.ItineraryByToken(ctx, hashToken(token))
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	s.attachPayer(ctx, paid)
	_ = s.sendReceipt(ctx, paid)
	return paid, nil
}

// Assign links a paid itinerary to a tourist account so it shows on their Home.
func (s *Service) Assign(ctx context.Context, userID, role, itineraryID, touristID string) (Itinerary, error) {
	op, err := s.operator(ctx, userID, role)
	if err != nil {
		return Itinerary{}, err
	}
	item, err := s.store.Itinerary(ctx, op.OrganisationID, itineraryID)
	if errors.Is(err, errNotFound) {
		return Itinerary{}, middleware.NotFound("not_found", "that itinerary was not found")
	}
	if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	if item.Status != "paid" {
		return Itinerary{}, middleware.Unprocessable("not_paid", "assign the tourist after the itinerary is paid")
	}
	if _, err := s.store.TouristID(ctx, touristID); errors.Is(err, errNotFound) {
		return Itinerary{}, middleware.NotFound("not_found", "that tourist account was not found")
	} else if err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.store.AssignTourist(ctx, item.ID, touristID, s.now()); err != nil {
		return Itinerary{}, middleware.Internal("internal_error", "internal server error")
	}
	item.TouristUserID = touristID
	return item, nil
}

func (s *Service) attachPayer(ctx context.Context, item Itinerary) {
	touristID, err := s.store.TouristIDByEmail(ctx, item.CustomerEmail)
	if err != nil {
		return
	}
	_ = s.store.AssignTourist(ctx, item.ID, touristID, s.now())
}

func (s *Service) sendReceipt(ctx context.Context, item Itinerary) error {
	paidOn := ""
	if item.PaidAt != nil {
		paidOn = item.PaidAt.Format("2 Jan 2006, 15:04")
	}
	receipt := appmail.Receipt{
		InvoiceNumber: item.InvoiceNumber, PaidOn: paidOn, CustomerName: item.CustomerName,
		BusinessName: item.BusinessName, Currency: item.Currency, Amount: item.AmountDue,
	}
	html, err := appmail.RenderReceipt(receipt)
	if err != nil {
		return err
	}
	filename := item.InvoiceNumber
	if filename == "" {
		filename = "receipt"
	}
	text := fmt.Sprintf("Payment received for %s.\nBilled to %s\nReceived by %s\nProcessed by VirungaPay\n%s %s\nThe PDF receipt is attached.\n", item.InvoiceNumber, item.CustomerName, item.BusinessName, item.Currency, item.AmountDue)
	return s.mail.Send(ctx, appmail.Message{
		ToEmail: item.CustomerEmail, ToName: item.CustomerName,
		Subject: "Receipt " + item.InvoiceNumber,
		Text:    text, HTML: html,
		Attachments: []appmail.Attachment{{
			Filename: filename + ".pdf", ContentType: "application/pdf", Content: appmail.ReceiptPDF(receipt),
		}},
	})
}

func (s *Service) operator(ctx context.Context, userID, role string) (Operator, error) {
	if role != auth.RoleOperatorOwner {
		return Operator{}, middleware.Forbidden("forbidden", "only a tour operator can do this")
	}
	op, err := s.store.Operator(ctx, userID)
	if errors.Is(err, errNotFound) {
		return Operator{}, middleware.NotFound("company_not_found", "create a company first")
	}
	if err != nil {
		return Operator{}, middleware.Internal("internal_error", "internal server error")
	}
	if op.Type != "operator" || op.Role != auth.RoleOperatorOwner {
		return Operator{}, middleware.Forbidden("forbidden", "only a tour operator can do this")
	}
	return op, nil
}

func normalizeInquiry(inquiry Inquiry) (Inquiry, error) {
	inquiry.CustomerName = strings.TrimSpace(inquiry.CustomerName)
	inquiry.CustomerEmail = strings.ToLower(strings.TrimSpace(inquiry.CustomerEmail))
	inquiry.Notes = strings.TrimSpace(inquiry.Notes)
	if inquiry.CustomerName == "" {
		return Inquiry{}, middleware.Invalid("invalid_request", "enter the customer's name")
	}
	if _, err := mail.ParseAddress(inquiry.CustomerEmail); err != nil {
		return Inquiry{}, middleware.Invalid("invalid_request", "enter the customer's email")
	}
	if _, _, err := dates(inquiry.StartsOn, inquiry.EndsOn); err != nil {
		return Inquiry{}, err
	}
	if inquiry.PartySize < 1 {
		return Inquiry{}, middleware.Invalid("invalid_request", "enter how many people are travelling")
	}
	return inquiry, nil
}

func normalizeItinerary(item Itinerary) (Itinerary, error) {
	item.Title = strings.TrimSpace(item.Title)
	item.Summary = strings.TrimSpace(item.Summary)
	item.CustomerName = strings.TrimSpace(item.CustomerName)
	item.CustomerEmail = strings.ToLower(strings.TrimSpace(item.CustomerEmail))
	item.Currency = strings.ToUpper(strings.TrimSpace(item.Currency))
	if item.Title == "" {
		return Itinerary{}, middleware.Invalid("invalid_request", "give the itinerary a title")
	}
	if item.CustomerName == "" {
		return Itinerary{}, middleware.Invalid("invalid_request", "enter the customer's name")
	}
	if _, err := mail.ParseAddress(item.CustomerEmail); err != nil {
		return Itinerary{}, middleware.Invalid("invalid_request", "enter the customer's email")
	}
	if _, _, err := dates(item.StartsOn, item.EndsOn); err != nil {
		return Itinerary{}, err
	}
	if item.PartySize < 1 {
		return Itinerary{}, middleware.Invalid("invalid_request", "enter how many people are travelling")
	}
	if item.Currency != "" && item.Currency != "USD" && item.Currency != "EUR" {
		return Itinerary{}, middleware.Invalid("invalid_request", "choose USD or EUR")
	}
	amount := strings.TrimSpace(item.AmountDue)
	if amount == "" {
		amount = "0"
	}
	parsed, ok := new(big.Rat).SetString(amount)
	if !ok || parsed.Sign() < 0 {
		return Itinerary{}, middleware.Invalid("invalid_request", "enter the amount due as a number")
	}
	item.AmountDue = parsed.FloatString(2)
	item.Days = cleanDays(item.Days)
	item.Inclusions = cleanLines(item.Inclusions)
	item.Exclusions = cleanLines(item.Exclusions)
	item.Notes = cleanLines(item.Notes)
	return item, nil
}

func dates(start, end string) (time.Time, time.Time, error) {
	starts, err := time.Parse("2006-01-02", start)
	if err != nil {
		return time.Time{}, time.Time{}, middleware.Invalid("invalid_request", "enter the start date")
	}
	ends, err := time.Parse("2006-01-02", end)
	if err != nil || ends.Before(starts) {
		return time.Time{}, time.Time{}, middleware.Invalid("invalid_request", "the end date must be on or after the start date")
	}
	return starts, ends, nil
}

func cleanDays(days []Day) []Day {
	out := make([]Day, 0, len(days))
	for i, day := range days {
		title := strings.TrimSpace(day.Title)
		if title == "" {
			continue
		}
		number := day.DayNumber
		if number < 1 {
			number = i + 1
		}
		out = append(out, Day{DayNumber: number, Title: title, Body: strings.TrimSpace(day.Body)})
	}
	return out
}

func cleanLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func mailData(item Itinerary, payURL, signupURL string) appmail.ItineraryMail {
	days := make([]appmail.ItineraryDay, 0, len(item.Days))
	for _, day := range item.Days {
		days = append(days, appmail.ItineraryDay{DayNumber: fmt.Sprintf("%d", day.DayNumber), Title: day.Title, Body: day.Body})
	}
	return appmail.ItineraryMail{
		BusinessName: item.BusinessName, CustomerName: item.CustomerName, Title: item.Title, Summary: item.Summary,
		StartsOn: item.StartsOn, EndsOn: item.EndsOn, PartySize: fmt.Sprintf("%d", item.PartySize),
		Currency: item.Currency, Amount: item.AmountDue, InvoiceNumber: item.InvoiceNumber, PayURL: payURL, SignupURL: signupURL,
		Days: days, Inclusions: item.Inclusions, Exclusions: item.Exclusions, Notes: item.Notes,
	}
}

func newToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
