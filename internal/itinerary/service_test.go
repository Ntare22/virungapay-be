package itinerary

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"virungapay-be/internal/auth"
	"virungapay-be/internal/company"
	"virungapay-be/internal/mail"
)

func TestSendCopiesNothingPricedAndPayOnce(t *testing.T) {
	store := newMem()
	box := &mail.Recorder{}
	payer := &fakePayer{}
	svc := NewService(store, payer, box, func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(nil)
	}, "http://app.test")
	ctx := context.Background()
	store.op = Operator{OrganisationID: "org-1", Name: "Kigezi Highland Safaris", Status: "approved", Type: "operator", Role: auth.RoleOperatorOwner}

	pkg, err := svc.SavePackage(ctx, "owner-1", auth.RoleOperatorOwner, Package{
		Title: "3 days gorilla safari", Summary: "Bwindi", Status: "published",
		Days:       []Day{{DayNumber: 1, Title: "Kampala to Bwindi", Body: "Equator stop"}},
		Inclusions: []string{"Gorilla permit"}, Exclusions: []string{"Tips"}, Notes: []string{"Maximum 6 people"},
	})
	require.NoError(t, err)

	inquiry, err := svc.CreateInquiry(ctx, "owner-1", auth.RoleOperatorOwner, Inquiry{
		CustomerName: "Amelie Laurent", CustomerEmail: "amelie@example.com",
		StartsOn: "2026-11-01", EndsOn: "2026-11-03", PartySize: 2, Notes: "Prefer a smaller group",
	})
	require.NoError(t, err)

	item, err := svc.SaveItinerary(ctx, "owner-1", auth.RoleOperatorOwner, Itinerary{
		PackageID: pkg.ID, InquiryID: inquiry.ID, Currency: "USD", AmountDue: "1240",
	})
	require.NoError(t, err)
	require.Equal(t, "3 days gorilla safari", item.Title)
	require.Equal(t, "Amelie Laurent", item.CustomerName)
	require.Equal(t, "Gorilla permit", item.Inclusions[0])
	require.Equal(t, "1240.00", item.AmountDue)
	require.Empty(t, item.InvoiceNumber)

	sent, err := svc.Send(ctx, "owner-1", auth.RoleOperatorOwner, item.ID)
	require.NoError(t, err)
	require.Equal(t, "sent", sent.Status)
	require.True(t, strings.HasPrefix(sent.InvoiceNumber, "VP-"))
	require.Contains(t, box.Messages[0].Text, sent.InvoiceNumber)
	require.Contains(t, box.Messages[0].Text, "/pay/")
	require.NotContains(t, pkg.Title, "1240")

	token := payToken(box.Messages[0].Text)
	paid, err := svc.Pay(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "paid", paid.Status)
	require.Equal(t, "org-1", payer.org)
	require.Equal(t, "1240.00", payer.amount)
	_, err = svc.Pay(ctx, token)
	require.Error(t, err)
	require.Equal(t, 1, payer.calls)
	require.Len(t, box.Messages, 2)
	require.Equal(t, "Receipt "+sent.InvoiceNumber, box.Messages[1].Subject)
	require.Equal(t, "amelie@example.com", box.Messages[1].ToEmail)
	require.Len(t, box.Messages[1].Attachments, 1)
	require.True(t, strings.HasPrefix(box.Messages[1].Attachments[0].Filename, "VP-"))
	require.True(t, strings.HasPrefix(string(box.Messages[1].Attachments[0].Content), "%PDF"))
	require.Contains(t, string(box.Messages[1].Attachments[0].Content), sent.InvoiceNumber)
}

type fakePayer struct {
	org, currency, amount string
	calls                 int
}

func (f *fakePayer) ReadyToCollect(context.Context, string, string) error { return nil }

func (f *fakePayer) CollectPayment(_ context.Context, organisationID, currency, amount string, _ company.CardPayment, after func(tx *gorm.DB) error) error {
	f.calls++
	f.org, f.currency, f.amount = organisationID, currency, amount
	return after(nil)
}

func payToken(text string) string {
	const marker = "/pay/"
	index := strings.LastIndex(text, marker)
	if index < 0 {
		return ""
	}
	token := text[index+len(marker):]
	return strings.TrimSpace(strings.Split(token, "\n")[0])
}

type memStore struct {
	op         Operator
	packages   map[string]Package
	inquiries  map[string]Inquiry
	items      map[string]Itinerary
	byToken    map[string]string
	invoiceSeq int
}

func newMem() *memStore {
	return &memStore{packages: map[string]Package{}, inquiries: map[string]Inquiry{}, items: map[string]Itinerary{}, byToken: map[string]string{}}
}

func (m *memStore) Operator(context.Context, string) (Operator, error) {
	if m.op.OrganisationID == "" {
		return Operator{}, errNotFound
	}
	return m.op, nil
}

func (m *memStore) ListPackages(context.Context, string) ([]Package, error) { return nil, nil }
func (m *memStore) Package(_ context.Context, _ *gorm.DB, _, id string) (Package, error) {
	pkg, ok := m.packages[id]
	if !ok {
		return Package{}, errNotFound
	}
	return pkg, nil
}
func (m *memStore) SavePackage(_ context.Context, _ *gorm.DB, pkg Package, _ time.Time) error {
	m.packages[pkg.ID] = pkg
	return nil
}
func (m *memStore) ListInquiries(context.Context, string) ([]Inquiry, error) { return nil, nil }
func (m *memStore) Inquiry(_ context.Context, _, id string) (Inquiry, error) {
	inquiry, ok := m.inquiries[id]
	if !ok {
		return Inquiry{}, errNotFound
	}
	return inquiry, nil
}
func (m *memStore) InsertInquiry(_ context.Context, inquiry Inquiry) error {
	m.inquiries[inquiry.ID] = inquiry
	return nil
}
func (m *memStore) ListItineraries(context.Context, string) ([]Itinerary, error) { return nil, nil }
func (m *memStore) Itinerary(_ context.Context, _, id string) (Itinerary, error) {
	item, ok := m.items[id]
	if !ok {
		return Itinerary{}, errNotFound
	}
	return item, nil
}
func (m *memStore) ItineraryByToken(_ context.Context, hash string) (Itinerary, error) {
	id, ok := m.byToken[hash]
	if !ok {
		return Itinerary{}, errNotFound
	}
	return m.items[id], nil
}
func (m *memStore) SaveItinerary(_ context.Context, _ *gorm.DB, item Itinerary, _ time.Time) error {
	m.items[item.ID] = item
	return nil
}
func (m *memStore) NextInvoice(context.Context, *gorm.DB, string, int) (string, error) {
	m.invoiceSeq++
	return "VP-2026-0001", nil
}
func (m *memStore) MarkSent(_ context.Context, _ *gorm.DB, id, invoice, tokenHash, currency, amount string, at time.Time) error {
	item := m.items[id]
	item.Status = "sent"
	item.InvoiceNumber = invoice
	item.PayTokenHash = tokenHash
	item.Currency = currency
	item.AmountDue = amount
	item.SentAt = &at
	m.items[id] = item
	m.byToken[tokenHash] = id
	return nil
}
func (m *memStore) RevertSent(context.Context, string, time.Time) error { return nil }
func (m *memStore) MarkPaid(_ context.Context, _ *gorm.DB, id string, at time.Time) error {
	item := m.items[id]
	if item.Status != "sent" {
		return errNotFound
	}
	item.Status = "paid"
	item.PaidAt = &at
	m.items[id] = item
	return nil
}
func (m *memStore) DB() *gorm.DB { return nil }
