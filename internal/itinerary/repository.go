package itinerary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errNotFound = errors.New("not found")

// Repository stores packages, inquiries, and itineraries.
type Repository struct {
	db *gorm.DB
}

// NewRepository uses the application GORM handle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB { return r.db }

type operatorRow struct {
	OrganisationID string
	Name           string
	Status         string
	Type           string
	Role           string `gorm:"column:member_role"`
}

func (r *Repository) Operator(ctx context.Context, userID string) (Operator, error) {
	var row operatorRow
	err := r.db.WithContext(ctx).Raw(`
SELECT m.organisation_id, o.name, o.status, o.type, m.member_role
FROM organisation_members m
JOIN organisations o ON o.id = m.organisation_id
WHERE m.user_id = ?
`, userID).Scan(&row).Error
	if err != nil {
		return Operator{}, err
	}
	if row.OrganisationID == "" {
		return Operator{}, errNotFound
	}
	return Operator{OrganisationID: row.OrganisationID, Name: row.Name, Status: row.Status, Type: row.Type, Role: row.Role}, nil
}

type packageRow struct {
	ID             string
	OrganisationID string
	Title          string
	Summary        string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (packageRow) TableName() string { return "packages" }

type dayRow struct {
	DayNumber int
	Title     string
	Body      string
}

type itemRow struct {
	Kind     string
	Body     string
	Position int
}

func (r *Repository) ListPackages(ctx context.Context, organisationID string) ([]Package, error) {
	var rows []packageRow
	err := r.db.WithContext(ctx).Where("organisation_id = ?", organisationID).Order("updated_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	packages := make([]Package, 0, len(rows))
	for _, row := range rows {
		pkg, err := r.packageFrom(ctx, r.db, row)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

func (r *Repository) Package(ctx context.Context, db *gorm.DB, organisationID, id string) (Package, error) {
	if db == nil {
		db = r.db
	}
	var row packageRow
	err := db.WithContext(ctx).Where("id = ? AND organisation_id = ?", id, organisationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Package{}, errNotFound
	}
	if err != nil {
		return Package{}, err
	}
	return r.packageFrom(ctx, db, row)
}

func (r *Repository) packageFrom(ctx context.Context, db *gorm.DB, row packageRow) (Package, error) {
	days, items, err := r.children(ctx, db, "package", row.ID)
	if err != nil {
		return Package{}, err
	}
	inclusions, exclusions, notes := splitItems(items)
	return Package{
		ID: row.ID, Title: row.Title, Summary: row.Summary, Status: row.Status,
		Days: days, Inclusions: inclusions, Exclusions: exclusions, Notes: notes, Organisation: row.OrganisationID,
	}, nil
}

func (r *Repository) SavePackage(ctx context.Context, db *gorm.DB, pkg Package, at time.Time) error {
	if pkg.ID == "" {
		pkg.ID = uuid.NewString()
	}
	row := packageRow{
		ID: pkg.ID, OrganisationID: pkg.Organisation, Title: pkg.Title, Summary: pkg.Summary, Status: pkg.Status,
		CreatedAt: at, UpdatedAt: at,
	}
	err := db.WithContext(ctx).Where("id = ? AND organisation_id = ?", pkg.ID, pkg.Organisation).First(&packageRow{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := db.WithContext(ctx).Model(&packageRow{}).Where("id = ?", pkg.ID).Updates(map[string]any{
		"title": pkg.Title, "summary": pkg.Summary, "status": pkg.Status, "updated_at": at,
	}).Error; err != nil {
		return err
	}
	return r.replaceChildren(ctx, db, "package", pkg.ID, pkg.Days, joinItems(pkg.Inclusions, pkg.Exclusions, pkg.Notes))
}

type inquiryRow struct {
	ID             string
	OrganisationID string
	CustomerName   string
	CustomerEmail  string
	StartsOn       string `gorm:"type:date"`
	EndsOn         string `gorm:"type:date"`
	PartySize      int
	Notes          string
	CreatedAt      time.Time
}

func (inquiryRow) TableName() string { return "inquiries" }

func (r *Repository) ListInquiries(ctx context.Context, organisationID string) ([]Inquiry, error) {
	var rows []inquiryRow
	err := r.db.WithContext(ctx).Where("organisation_id = ?", organisationID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]Inquiry, 0, len(rows))
	for _, row := range rows {
		list = append(list, inquiryFrom(row))
	}
	return list, nil
}

func (r *Repository) Inquiry(ctx context.Context, organisationID, id string) (Inquiry, error) {
	var row inquiryRow
	err := r.db.WithContext(ctx).Where("id = ? AND organisation_id = ?", id, organisationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Inquiry{}, errNotFound
	}
	if err != nil {
		return Inquiry{}, err
	}
	return inquiryFrom(row), nil
}

func (r *Repository) InsertInquiry(ctx context.Context, inquiry Inquiry) error {
	row := inquiryRow{
		ID: inquiry.ID, OrganisationID: inquiry.Organisation, CustomerName: inquiry.CustomerName,
		CustomerEmail: inquiry.CustomerEmail, StartsOn: inquiry.StartsOn, EndsOn: inquiry.EndsOn,
		PartySize: inquiry.PartySize, Notes: inquiry.Notes, CreatedAt: inquiry.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func inquiryFrom(row inquiryRow) Inquiry {
	return Inquiry{
		ID: row.ID, CustomerName: row.CustomerName, CustomerEmail: row.CustomerEmail,
		StartsOn: dateOnly(row.StartsOn), EndsOn: dateOnly(row.EndsOn), PartySize: row.PartySize,
		Notes: row.Notes, Organisation: row.OrganisationID, CreatedAt: row.CreatedAt,
	}
}

type itineraryRow struct {
	ID             string
	OrganisationID string
	PackageID      *string
	InquiryID      *string
	CustomerName   string
	CustomerEmail  string
	StartsOn       string `gorm:"type:date"`
	EndsOn         string `gorm:"type:date"`
	PartySize      int
	Title          string
	Summary        string
	Currency       string
	AmountDue      string `gorm:"type:numeric(20,2)"`
	InvoiceNumber  *string
	PayTokenHash   *string
	TouristUserID  *string
	Status         string
	SentAt         *time.Time
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (itineraryRow) TableName() string { return "itineraries" }

func (r *Repository) ListItineraries(ctx context.Context, organisationID string) ([]Itinerary, error) {
	var rows []itineraryRow
	err := r.db.WithContext(ctx).Where("organisation_id = ?", organisationID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]Itinerary, 0, len(rows))
	for _, row := range rows {
		item, err := r.itineraryFrom(ctx, r.db, row, "")
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) UserEmail(ctx context.Context, userID string) (string, error) {
	var email string
	err := r.db.WithContext(ctx).Raw(`SELECT email FROM users WHERE id = ?`, userID).Scan(&email).Error
	if err != nil {
		return "", err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errNotFound
	}
	return email, nil
}

func (r *Repository) ListMine(ctx context.Context, userID, email string) ([]Itinerary, error) {
	var rows []itineraryRow
	err := r.db.WithContext(ctx).
		Where("status = ? AND (tourist_user_id = ? OR (tourist_user_id IS NULL AND lower(customer_email) = ?))", "paid", userID, strings.ToLower(strings.TrimSpace(email))).
		Order("paid_at DESC NULLS LAST, created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	list := make([]Itinerary, 0, len(rows))
	for _, row := range rows {
		name, ok := names[row.OrganisationID]
		if !ok {
			_ = r.db.WithContext(ctx).Raw(`SELECT name FROM organisations WHERE id = ?`, row.OrganisationID).Scan(&name).Error
			names[row.OrganisationID] = name
		}
		item, err := r.itineraryFrom(ctx, r.db, row, name)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) TouristID(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Raw(`SELECT id FROM users WHERE id = ? AND role = 'tourist'`, userID).Scan(&id).Error
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", errNotFound
	}
	return id, nil
}

func (r *Repository) TouristIDByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Raw(`SELECT id FROM users WHERE role = 'tourist' AND lower(email) = ?`, strings.ToLower(strings.TrimSpace(email))).Scan(&id).Error
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", errNotFound
	}
	return id, nil
}

func (r *Repository) AssignTourist(ctx context.Context, itineraryID, touristID string, at time.Time) error {
	result := r.db.WithContext(ctx).Model(&itineraryRow{}).Where("id = ? AND status = ?", itineraryID, "paid").Updates(map[string]any{
		"tourist_user_id": touristID, "updated_at": at,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) Itinerary(ctx context.Context, organisationID, id string) (Itinerary, error) {
	var row itineraryRow
	err := r.db.WithContext(ctx).Where("id = ? AND organisation_id = ?", id, organisationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Itinerary{}, errNotFound
	}
	if err != nil {
		return Itinerary{}, err
	}
	return r.itineraryFrom(ctx, r.db, row, "")
}

func (r *Repository) ItineraryByToken(ctx context.Context, hash string) (Itinerary, error) {
	var row itineraryRow
	err := r.db.WithContext(ctx).Where("pay_token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Itinerary{}, errNotFound
	}
	if err != nil {
		return Itinerary{}, err
	}
	var name string
	_ = r.db.WithContext(ctx).Raw(`SELECT name FROM organisations WHERE id = ?`, row.OrganisationID).Scan(&name).Error
	return r.itineraryFrom(ctx, r.db, row, name)
}

func (r *Repository) SaveItinerary(ctx context.Context, db *gorm.DB, item Itinerary, at time.Time) error {
	row := itineraryRow{
		ID: item.ID, OrganisationID: item.Organisation, CustomerName: item.CustomerName, CustomerEmail: item.CustomerEmail,
		StartsOn: item.StartsOn, EndsOn: item.EndsOn, PartySize: item.PartySize, Title: item.Title, Summary: item.Summary,
		Currency: item.Currency, AmountDue: item.AmountDue, Status: item.Status, CreatedAt: at, UpdatedAt: at,
	}
	if item.PackageID != "" {
		row.PackageID = &item.PackageID
	}
	if item.InquiryID != "" {
		row.InquiryID = &item.InquiryID
	}
	err := db.WithContext(ctx).Where("id = ? AND organisation_id = ?", item.ID, item.Organisation).First(&itineraryRow{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := db.WithContext(ctx).Model(&itineraryRow{}).Where("id = ? AND status = ?", item.ID, "draft").Updates(map[string]any{
		"customer_name": item.CustomerName, "customer_email": item.CustomerEmail, "starts_on": item.StartsOn, "ends_on": item.EndsOn,
		"party_size": item.PartySize, "title": item.Title, "summary": item.Summary, "currency": item.Currency,
		"amount_due": item.AmountDue, "updated_at": at,
	}).Error; err != nil {
		return err
	}
	return r.replaceChildren(ctx, db, "itinerary", item.ID, item.Days, joinItems(item.Inclusions, item.Exclusions, item.Notes))
}

func (r *Repository) NextInvoice(ctx context.Context, db *gorm.DB, organisationID string, year int) (string, error) {
	prefix := fmt.Sprintf("VP-%d-", year)
	var last string
	err := db.WithContext(ctx).Raw(`
SELECT COALESCE(invoice_number, '') FROM itineraries
WHERE organisation_id = ? AND invoice_number LIKE ?
ORDER BY invoice_number DESC
LIMIT 1
`, organisationID, prefix+"%").Scan(&last).Error
	if err != nil {
		return "", err
	}
	next := 1
	if last != "" {
		fmt.Sscanf(strings.TrimPrefix(last, prefix), "%d", &next)
		next++
	}
	return fmt.Sprintf("%s%04d", prefix, next), nil
}

func (r *Repository) MarkSent(ctx context.Context, db *gorm.DB, id, invoice, tokenHash, currency, amount string, at time.Time) error {
	result := db.WithContext(ctx).Model(&itineraryRow{}).Where("id = ? AND status = ?", id, "draft").Updates(map[string]any{
		"status": "sent", "invoice_number": invoice, "pay_token_hash": tokenHash,
		"currency": currency, "amount_due": amount, "sent_at": at, "updated_at": at,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) RevertSent(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&itineraryRow{}).Where("id = ? AND status = ? AND paid_at IS NULL", id, "sent").Updates(map[string]any{
		"status": "draft", "invoice_number": nil, "pay_token_hash": nil, "sent_at": nil, "updated_at": at,
	}).Error
}

func (r *Repository) MarkPaid(ctx context.Context, db *gorm.DB, id string, at time.Time) error {
	result := db.WithContext(ctx).Model(&itineraryRow{}).Where("id = ? AND status = ?", id, "sent").Updates(map[string]any{
		"status": "paid", "paid_at": at, "updated_at": at,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) itineraryFrom(ctx context.Context, db *gorm.DB, row itineraryRow, business string) (Itinerary, error) {
	days, items, err := r.children(ctx, db, "itinerary", row.ID)
	if err != nil {
		return Itinerary{}, err
	}
	inclusions, exclusions, notes := splitItems(items)
	item := Itinerary{
		ID: row.ID, CustomerName: row.CustomerName, CustomerEmail: row.CustomerEmail,
		StartsOn: dateOnly(row.StartsOn), EndsOn: dateOnly(row.EndsOn), PartySize: row.PartySize,
		Title: row.Title, Summary: row.Summary, Currency: row.Currency, AmountDue: row.AmountDue,
		Status: row.Status, BusinessName: business, Days: days, Inclusions: inclusions, Exclusions: exclusions, Notes: notes,
		Organisation: row.OrganisationID, SentAt: row.SentAt, PaidAt: row.PaidAt,
	}
	if row.TouristUserID != nil {
		item.TouristUserID = *row.TouristUserID
	}
	if row.PackageID != nil {
		item.PackageID = *row.PackageID
	}
	if row.InquiryID != nil {
		item.InquiryID = *row.InquiryID
	}
	if row.InvoiceNumber != nil {
		item.InvoiceNumber = *row.InvoiceNumber
	}
	if row.PayTokenHash != nil {
		item.PayTokenHash = *row.PayTokenHash
	}
	return item, nil
}

func (r *Repository) children(ctx context.Context, db *gorm.DB, parent, id string) ([]Day, []storedItem, error) {
	dayTable := parent + "_days"
	itemTable := parent + "_items"
	dayColumn := parent + "_id"
	var days []dayRow
	if err := db.WithContext(ctx).Table(dayTable).Where(dayColumn+" = ?", id).Order("day_number").Find(&days).Error; err != nil {
		return nil, nil, err
	}
	var items []itemRow
	if err := db.WithContext(ctx).Table(itemTable).Where(dayColumn+" = ?", id).Order("position").Find(&items).Error; err != nil {
		return nil, nil, err
	}
	outDays := make([]Day, 0, len(days))
	for _, day := range days {
		outDays = append(outDays, Day{DayNumber: day.DayNumber, Title: day.Title, Body: day.Body})
	}
	outItems := make([]storedItem, 0, len(items))
	for _, item := range items {
		outItems = append(outItems, storedItem{Kind: item.Kind, Body: item.Body})
	}
	return outDays, outItems, nil
}

func (r *Repository) replaceChildren(ctx context.Context, db *gorm.DB, parent, id string, days []Day, items []storedItem) error {
	dayTable := parent + "_days"
	itemTable := parent + "_items"
	column := parent + "_id"
	if err := db.WithContext(ctx).Exec("DELETE FROM "+dayTable+" WHERE "+column+" = ?", id).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec("DELETE FROM "+itemTable+" WHERE "+column+" = ?", id).Error; err != nil {
		return err
	}
	for _, day := range days {
		row := map[string]any{"id": uuid.NewString(), column: id, "day_number": day.DayNumber, "title": day.Title, "body": day.Body}
		if err := db.WithContext(ctx).Table(dayTable).Create(row).Error; err != nil {
			return err
		}
	}
	for i, item := range items {
		row := map[string]any{"id": uuid.NewString(), column: id, "kind": item.Kind, "body": item.Body, "position": i}
		if err := db.WithContext(ctx).Table(itemTable).Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

type storedItem struct {
	Kind string
	Body string
}

func splitItems(items []storedItem) (inclusions, exclusions, notes []string) {
	inclusions, exclusions, notes = []string{}, []string{}, []string{}
	for _, item := range items {
		switch item.Kind {
		case "inclusion":
			inclusions = append(inclusions, item.Body)
		case "exclusion":
			exclusions = append(exclusions, item.Body)
		default:
			notes = append(notes, item.Body)
		}
	}
	return inclusions, exclusions, notes
}

func joinItems(inclusions, exclusions, notes []string) []storedItem {
	items := make([]storedItem, 0, len(inclusions)+len(exclusions)+len(notes))
	for _, body := range inclusions {
		items = append(items, storedItem{Kind: "inclusion", Body: body})
	}
	for _, body := range exclusions {
		items = append(items, storedItem{Kind: "exclusion", Body: body})
	}
	for _, body := range notes {
		items = append(items, storedItem{Kind: "note", Body: body})
	}
	return items
}

func dateOnly(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}
