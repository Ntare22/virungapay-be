package mail

import (
	_ "embed"

	"github.com/aymerick/raymond"
)

//go:embed templates/itinerary.hbs
var itineraryTemplate string

// ItineraryDay is one day in the emailed plan.
type ItineraryDay struct {
	DayNumber string
	Title     string
	Body      string
}

// ItineraryMail is the invoice email sent to the customer.
type ItineraryMail struct {
	BusinessName  string
	CustomerName  string
	Title         string
	Summary       string
	StartsOn      string
	EndsOn        string
	PartySize     string
	Currency      string
	Amount        string
	InvoiceNumber string
	PayURL        string
	Days          []ItineraryDay
	Inclusions    []string
	Exclusions    []string
	Notes         []string
}

// RenderItinerary fills the itinerary invoice template.
func RenderItinerary(data ItineraryMail) (string, error) {
	days := make([]map[string]string, 0, len(data.Days))
	for _, day := range data.Days {
		days = append(days, map[string]string{"dayNumber": day.DayNumber, "title": day.Title, "body": day.Body})
	}
	return raymond.Render(itineraryTemplate, map[string]any{
		"businessName": data.BusinessName, "customerName": data.CustomerName, "title": data.Title,
		"summary": data.Summary, "startsOn": data.StartsOn, "endsOn": data.EndsOn, "partySize": data.PartySize,
		"currency": data.Currency, "amount": data.Amount, "invoiceNumber": data.InvoiceNumber, "payURL": data.PayURL,
		"days": days, "inclusions": data.Inclusions, "exclusions": data.Exclusions,
	})
}
