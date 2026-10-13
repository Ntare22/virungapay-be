// Package itinerary stores curated packages, customer inquiries, and the itinerary that is emailed and paid.
package itinerary

import "time"

// Day is one day of a package or itinerary. It has no price.
type Day struct {
	DayNumber int    `json:"day_number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}

// Package is a reusable trip an operator curates. It has no price.
type Package struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Status       string   `json:"status"`
	Days         []Day    `json:"days"`
	Inclusions   []string `json:"inclusions"`
	Exclusions   []string `json:"exclusions"`
	Notes        []string `json:"notes"`
	Organisation string   `json:"-"`
}

// Inquiry is a customer request the operator recorded.
type Inquiry struct {
	ID            string    `json:"id"`
	CustomerName  string    `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	StartsOn      string    `json:"starts_on"`
	EndsOn        string    `json:"ends_on"`
	PartySize     int       `json:"party_size"`
	Notes         string    `json:"notes"`
	Organisation  string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

// Itinerary is the document sent to one customer, with one amount due.
type Itinerary struct {
	ID            string     `json:"id"`
	PackageID     string     `json:"package_id,omitempty"`
	InquiryID     string     `json:"inquiry_id,omitempty"`
	CustomerName  string     `json:"customer_name"`
	CustomerEmail string     `json:"customer_email"`
	StartsOn      string     `json:"starts_on"`
	EndsOn        string     `json:"ends_on"`
	PartySize     int        `json:"party_size"`
	Title         string     `json:"title"`
	Summary       string     `json:"summary"`
	Currency      string     `json:"currency"`
	AmountDue     string     `json:"amount_due"`
	InvoiceNumber string     `json:"invoice_number,omitempty"`
	Status        string     `json:"status"`
	TouristUserID string     `json:"tourist_user_id,omitempty"`
	BusinessName  string     `json:"business_name,omitempty"`
	Days          []Day      `json:"days"`
	Inclusions    []string   `json:"inclusions"`
	Exclusions    []string   `json:"exclusions"`
	Notes         []string   `json:"notes"`
	Organisation  string     `json:"-"`
	PayTokenHash  string     `json:"-"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}

// Operator is the signed-in tour operator's company.
type Operator struct {
	OrganisationID string
	Name           string
	Status         string
	Type           string
	Role           string
}
