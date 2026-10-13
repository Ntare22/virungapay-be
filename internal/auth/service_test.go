package auth

import (
	"context"
	"errors"
	"testing"
	"time"
	"virungapay-be/internal/middleware"

	"github.com/stretchr/testify/require"

	"virungapay-be/internal/mail"
)

func TestRegisterTouristSendsVerificationWithoutSession(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	wallets := &memWallets{}
	svc := NewService(store, nopInvites{}, nopMembers{}, wallets, box, "secret", "http://app.test")

	user, err := svc.RegisterTourist(context.Background(), Signup{
		FirstName: "amelie", LastName: "laurent", Email: "Amelie@Example.com", Password: "password1",
	})
	require.NoError(t, err)
	require.Equal(t, RoleTourist, user.Role)
	require.Nil(t, user.EmailVerifiedAt)
	require.Equal(t, "amelie@example.com", user.Email)
	require.Empty(t, user.Phone)
	require.Equal(t, []string{user.ID}, wallets.userIDs)
	require.Len(t, box.Messages, 1)
	require.Equal(t, "Your VirungaPay account", box.Messages[0].Subject)
	require.Contains(t, box.Messages[0].HTML, "Hello, Amelie.")
	require.Contains(t, box.Messages[0].HTML, "East Africa · VirungaPay")
	require.Empty(t, box.TokenFor("amelie@example.com"))
	require.NotContains(t, box.Messages[0].Text, "password")
	require.NotContains(t, user.PasswordHash, "password1")
}

func TestRegisterOperatorSendsWelcomeEmail(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	wallets := &memWallets{}
	svc := NewService(store, nopInvites{}, nopMembers{}, wallets, box, "secret", "http://app.test")

	_, err := svc.RegisterOperator(context.Background(), Signup{
		FirstName: "amina", LastName: "kato", Email: "amina@example.com", Phone: "+256700000000", Password: "password1",
	})
	require.NoError(t, err)
	require.Len(t, box.Messages, 1)
	require.Equal(t, "Welcome to VirungaPay", box.Messages[0].Subject)
	require.Contains(t, box.Messages[0].Text, "Welcome to VirungaPay, Amina Kato")
	require.Contains(t, box.Messages[0].HTML, "Hello, Amina.")
	require.Contains(t, box.Messages[0].HTML, "Verify email")
	require.NotEmpty(t, box.TokenFor("amina@example.com"))
	require.Empty(t, wallets.userIDs)
}

func TestUnverifiedLoginIsRejected(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	svc := NewService(store, nopInvites{}, nopMembers{}, nopWallets{}, box, "secret", "http://app.test")
	_, err := svc.RegisterOperator(context.Background(), Signup{FirstName: "Amina", LastName: "Kato", Email: "amina@example.com", Password: "password1"})
	require.NoError(t, err)

	_, err = svc.Login(context.Background(), Login{Email: "amina@example.com", Password: "password1"}, AudiencePWA)
	require.Error(t, err)
	var app *middleware.AppError
	require.ErrorAs(t, err, &app)
	require.Equal(t, "email_unverified", app.Code)
}

func TestResetTokenCannotBeReused(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	svc := NewService(store, nopInvites{}, nopMembers{}, nopWallets{}, box, "secret", "http://app.test")
	ctx := context.Background()
	_, err := svc.RegisterOperator(ctx, Signup{FirstName: "Amina", LastName: "Kato", Email: "amina@example.com", Password: "password1"})
	require.NoError(t, err)
	require.NoError(t, svc.VerifyEmail(ctx, box.TokenFor("amina@example.com")))
	require.NoError(t, svc.VerifyEmail(ctx, box.TokenFor("amina@example.com")))

	require.NoError(t, svc.ForgotPassword(ctx, "amina@example.com", AudiencePWA))
	token := box.TokenFor("amina@example.com")
	require.NotEmpty(t, token)
	require.NoError(t, svc.ResetPassword(ctx, ResetPassword{Token: token, Password: "password2"}, AudiencePWA))
	err = svc.ResetPassword(ctx, ResetPassword{Token: token, Password: "password3"}, AudiencePWA)
	require.Error(t, err)
	var app *middleware.AppError
	require.ErrorAs(t, err, &app)
	require.Equal(t, "invalid_token", app.Code)

	_, err = svc.Login(ctx, Login{Email: "amina@example.com", Password: "password2"}, AudiencePWA)
	require.NoError(t, err)
}

func TestEnsureStaffCanUseAdminLogin(t *testing.T) {
	store := newMemUsers()
	svc := NewService(store, nopInvites{}, nopMembers{}, nopWallets{}, &mail.Recorder{}, "secret", "http://app.test")
	ctx := context.Background()
	require.NoError(t, svc.EnsureStaff(ctx, "VirungaPay Admin", "admin@virungapay.demo", "password1"))
	require.NoError(t, svc.EnsureStaff(ctx, "VirungaPay Admin", "admin@virungapay.demo", "password1"))

	session, err := svc.Login(ctx, Login{Email: "admin@virungapay.demo", Password: "password1"}, AudienceAdmin)
	require.NoError(t, err)
	require.Equal(t, RoleVASuperadmin, session.User.Role)
	require.NotEmpty(t, session.Token)

	_, err = svc.Login(ctx, Login{Email: "admin@virungapay.demo", Password: "password1"}, AudiencePWA)
	require.Error(t, err)
}

func TestTouristAddsPhoneOnProfile(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	svc := NewService(store, nopInvites{}, nopMembers{}, nopWallets{}, box, "secret", "http://app.test")
	ctx := context.Background()
	user, err := svc.RegisterTourist(ctx, Signup{FirstName: "Amelie", LastName: "Laurent", Email: "amelie@example.com"})
	require.NoError(t, err)
	require.NoError(t, svc.RequestLoginCode(ctx, "amelie@example.com"))
	session, err := svc.ConfirmLoginCode(ctx, "amelie@example.com", box.CodeFor("amelie@example.com"))
	require.NoError(t, err)
	require.True(t, session.User.EmailVerified)

	profile, err := svc.UpdatePhone(ctx, user.ID, " +256700000000 ")
	require.NoError(t, err)
	require.Equal(t, "+256700000000", profile.Phone)

	_, err = svc.UpdatePhone(ctx, user.ID, "0700000000")
	require.Error(t, err)
	var app *middleware.AppError
	require.ErrorAs(t, err, &app)
	require.Equal(t, "invalid_phone", app.Code)

	other, err := svc.RegisterTourist(ctx, Signup{FirstName: "Noah", LastName: "Kato", Email: "noah@example.com", Password: "password1"})
	require.NoError(t, err)
	_, err = svc.UpdatePhone(ctx, other.ID, "+256700000000")
	require.ErrorAs(t, err, &app)
	require.Equal(t, "phone_taken", app.Code)
}

func TestPhoneLoginCodeSignsInOnce(t *testing.T) {
	store := newMemUsers()
	box := &mail.Recorder{}
	svc := NewService(store, nopInvites{}, nopMembers{}, nopWallets{}, box, "secret", "http://app.test")
	ctx := context.Background()
	_, err := svc.RegisterTourist(ctx, Signup{FirstName: "Amelie", LastName: "Laurent", Email: "amelie@example.com"})
	require.NoError(t, err)
	require.NoError(t, svc.RequestLoginCode(ctx, "missing@example.com"))
	require.Empty(t, box.CodeFor("missing@example.com"))
	require.NoError(t, svc.RequestLoginCode(ctx, "amelie@example.com"))
	code := box.CodeFor("amelie@example.com")
	require.Len(t, code, 6)
	require.Contains(t, box.Messages[len(box.Messages)-1].HTML, code)
	require.Contains(t, box.Messages[len(box.Messages)-1].HTML, "East Africa · VirungaPay")

	session, err := svc.ConfirmLoginCode(ctx, "amelie@example.com", code)
	require.NoError(t, err)
	require.Equal(t, RoleTourist, session.User.Role)
	require.True(t, session.User.EmailVerified)
	require.NotEmpty(t, session.Token)

	_, err = svc.ConfirmLoginCode(ctx, "amelie@example.com", code)
	require.Error(t, err)

	_, err = svc.RegisterOperator(ctx, Signup{FirstName: "Amina", LastName: "Kato", Email: "amina@example.com", Password: "password1"})
	require.NoError(t, err)
	require.NoError(t, svc.RequestLoginCode(ctx, "amina@example.com"))
	operator, err := svc.ConfirmLoginCode(ctx, "amina@example.com", box.CodeFor("amina@example.com"))
	require.NoError(t, err)
	require.Equal(t, RoleOperatorOwner, operator.User.Role)
}

func TestForgotPasswordHidesUnknownEmail(t *testing.T) {
	box := &mail.Recorder{}
	svc := NewService(newMemUsers(), nopInvites{}, nopMembers{}, nopWallets{}, box, "secret", "http://app.test")
	require.NoError(t, svc.ForgotPassword(context.Background(), "missing@example.com", AudiencePWA))
	require.Empty(t, box.Messages)
}

type memUsers struct {
	byID    map[string]User
	byEmail map[string]User
	emails  map[string]EmailToken
	resets  map[string]ResetToken
	codes   map[string]LoginCode
	latest  map[string]string
}

func newMemUsers() *memUsers {
	return &memUsers{
		byID: map[string]User{}, byEmail: map[string]User{},
		emails: map[string]EmailToken{}, resets: map[string]ResetToken{},
		codes: map[string]LoginCode{}, latest: map[string]string{},
	}
}

func (m *memUsers) Create(_ context.Context, user User) error {
	if _, ok := m.byEmail[user.Email]; ok {
		return errDuplicate
	}
	m.byID[user.ID] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) ByEmail(_ context.Context, email string) (User, error) {
	user, ok := m.byEmail[email]
	if !ok {
		return User{}, errNotFound
	}
	return user, nil
}

func (m *memUsers) ByID(_ context.Context, id string) (User, error) {
	user, ok := m.byID[id]
	if !ok {
		return User{}, errNotFound
	}
	return user, nil
}

func (m *memUsers) SetPassword(_ context.Context, id, hash string, at time.Time) error {
	user := m.byID[id]
	user.PasswordHash = hash
	user.UpdatedAt = at
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SetEmailVerified(_ context.Context, id string, at time.Time) error {
	user := m.byID[id]
	user.EmailVerifiedAt = &at
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SetPhone(_ context.Context, id, phone string, at time.Time) error {
	for _, existing := range m.byID {
		if existing.ID != id && existing.Phone == phone {
			return errDuplicate
		}
	}
	user := m.byID[id]
	user.Phone = phone
	user.UpdatedAt = at
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SetStatus(_ context.Context, id, status string, at time.Time) error {
	user := m.byID[id]
	user.Status = status
	user.UpdatedAt = at
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SaveEmailToken(_ context.Context, token EmailToken) error {
	m.emails[token.TokenHash] = token
	return nil
}

func (m *memUsers) EmailToken(_ context.Context, hash string) (EmailToken, error) {
	token, ok := m.emails[hash]
	if !ok {
		return EmailToken{}, errNotFound
	}
	return token, nil
}

func (m *memUsers) UseEmailToken(_ context.Context, id string, at time.Time) error {
	for hash, token := range m.emails {
		if token.ID == id {
			token.UsedAt = &at
			m.emails[hash] = token
		}
	}
	return nil
}

func (m *memUsers) SaveResetToken(_ context.Context, token ResetToken) error {
	m.resets[token.TokenHash] = token
	return nil
}

func (m *memUsers) ResetToken(_ context.Context, hash string) (ResetToken, error) {
	token, ok := m.resets[hash]
	if !ok {
		return ResetToken{}, errNotFound
	}
	return token, nil
}

func (m *memUsers) UseResetToken(_ context.Context, id string, at time.Time) error {
	for hash, token := range m.resets {
		if token.ID == id {
			token.UsedAt = &at
			m.resets[hash] = token
		}
	}
	return nil
}

func (m *memUsers) SaveLoginCode(_ context.Context, code LoginCode) error {
	m.codes[code.ID] = code
	m.latest[code.UserID] = code.ID
	return nil
}

func (m *memUsers) LatestLoginCode(_ context.Context, userID string) (LoginCode, error) {
	id, ok := m.latest[userID]
	if !ok {
		return LoginCode{}, errNotFound
	}
	return m.codes[id], nil
}

func (m *memUsers) UseLoginCode(_ context.Context, id string, at time.Time) error {
	code, ok := m.codes[id]
	if !ok {
		return errNotFound
	}
	code.UsedAt = &at
	m.codes[id] = code
	return nil
}

type nopInvites struct{}

func (nopInvites) FindByTokenHash(context.Context, string) (PendingInvite, error) {
	return PendingInvite{}, errors.New("missing")
}
func (nopInvites) MarkAccepted(context.Context, string, time.Time) error { return nil }

type nopMembers struct{}

func (nopMembers) OrganisationID(context.Context, string) (string, error) { return "", nil }

type nopWallets struct{}

func (nopWallets) OpenTouristWallet(context.Context, string) error { return nil }

type memWallets struct {
	userIDs []string
}

func (m *memWallets) OpenTouristWallet(_ context.Context, userID string) error {
	m.userIDs = append(m.userIDs, userID)
	return nil
}
