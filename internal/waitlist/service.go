package waitlist

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"virungapay-be/internal/middleware"
)

// Store saves a waitlist email. A repeat address is not an error.
type Store interface {
	Save(ctx context.Context, email string, at time.Time) error
}

// Service records people who want to hear when VirungaPay opens.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService builds the waitlist service.
func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

// Join adds an email to the waitlist. An address that is already there stays as it is.
func (s *Service) Join(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, ".") {
		return middleware.Invalid("invalid_request", "Enter the email where we should write to you.")
	}
	if err := s.store.Save(ctx, email, s.now()); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}
