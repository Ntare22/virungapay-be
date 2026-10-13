package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"virungapay-be/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"virungapay-be/internal/mail"
)

const (
	tokenTTL       = 24 * time.Hour
	verifyTTL      = 24 * time.Hour
	resetTTL       = time.Hour
	loginCodeTTL   = 10 * time.Minute
	passwordMinLen = 8
)

// phonePattern is an E.164 number: a leading plus and 8 to 15 digits.
var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// UserStore is the persistence used by the auth service.
type UserStore interface {
	Create(ctx context.Context, user User) error
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id string) (User, error)
	SetPassword(ctx context.Context, id, hash string, at time.Time) error
	SetEmailVerified(ctx context.Context, id string, at time.Time) error
	SetStatus(ctx context.Context, id, status string, at time.Time) error
	SetPhone(ctx context.Context, id, phone string, at time.Time) error
	SaveEmailToken(ctx context.Context, token EmailToken) error
	EmailToken(ctx context.Context, hash string) (EmailToken, error)
	UseEmailToken(ctx context.Context, id string, at time.Time) error
	SaveResetToken(ctx context.Context, token ResetToken) error
	ResetToken(ctx context.Context, hash string) (ResetToken, error)
	UseResetToken(ctx context.Context, id string, at time.Time) error
	SaveLoginCode(ctx context.Context, code LoginCode) error
	LatestLoginCode(ctx context.Context, userID string) (LoginCode, error)
	UseLoginCode(ctx context.Context, id string, at time.Time) error
}

// InviteStore looks up company invites. The company module owns the rows.
type InviteStore interface {
	FindByTokenHash(ctx context.Context, hash string) (PendingInvite, error)
	MarkAccepted(ctx context.Context, id string, at time.Time) error
}

// MembershipStore returns the company a person belongs to, if any.
type MembershipStore interface {
	OrganisationID(ctx context.Context, userID string) (string, error)
}

// TouristWallets opens the VirungaPass Wallet that belongs to a new tourist.
type TouristWallets interface {
	OpenTouristWallet(ctx context.Context, userID string) error
}

// Service registers people, issues sessions, and sends account email.
type Service struct {
	users   UserStore
	invites InviteStore
	members MembershipStore
	wallets TouristWallets
	mail    mail.Sender
	secret  []byte
	public  string
	now     func() time.Time
}

// NewService builds the auth service. public is the app origin used in links, without a trailing slash.
func NewService(users UserStore, invites InviteStore, members MembershipStore, wallets TouristWallets, sender mail.Sender, jwtSecret, publicURL string) *Service {
	return &Service{
		users:   users,
		invites: invites,
		members: members,
		wallets: wallets,
		mail:    sender,
		secret:  []byte(jwtSecret),
		public:  strings.TrimRight(publicURL, "/"),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// RegisterTourist creates a tourist with no password and emails that the phone app will send a code.
func (s *Service) RegisterTourist(ctx context.Context, in Signup) (User, error) {
	return s.register(ctx, in, RoleTourist, false)
}

// RegisterOperator creates an unverified operator owner. The company module is the only caller.
func (s *Service) RegisterOperator(ctx context.Context, in Signup) (User, error) {
	return s.register(ctx, in, RoleOperatorOwner, true)
}

// RegisterForexBureau creates an unverified forex bureau owner.
func (s *Service) RegisterForexBureau(ctx context.Context, in Signup) (User, error) {
	return s.register(ctx, in, RoleForexBureauOwner, true)
}

func (s *Service) register(ctx context.Context, in Signup, role string, requirePassword bool) (User, error) {
	if requirePassword {
		if err := validateSignup(in); err != nil {
			return User{}, err
		}
	} else if strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" || strings.TrimSpace(in.Email) == "" {
		return User{}, middleware.Invalid("invalid_request", "first name, last name, and email are required")
	} else {
		secret, err := randomSecret()
		if err != nil {
			return User{}, middleware.Internal("internal_error", "internal server error")
		}
		in.Password = secret
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, middleware.Internal("internal_error", "internal server error")
	}
	now := s.now()
	first, last := personName(in.FirstName, in.LastName)
	user := User{
		ID:           uuid.NewString(),
		Role:         role,
		FirstName:    first,
		LastName:     last,
		Email:        normalizeEmail(in.Email),
		Phone:        strings.TrimSpace(in.Phone),
		PasswordHash: string(hash),
		Status:       StatusActive,
		KYCStatus:    "none",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, errDuplicate) {
			return User{}, middleware.Conflict("email_taken", "an account with this email already exists")
		}
		return User{}, middleware.Internal("internal_error", "internal server error")
	}
	if role == RoleTourist {
		if err := s.wallets.OpenTouristWallet(ctx, user.ID); err != nil {
			return User{}, middleware.Internal("internal_error", "internal server error")
		}
	}
	if !requirePassword {
		body := "Your tourist account is ready. Sign in on your phone and we will email a code."
		rendered, err := mail.RenderStandard(mail.Standard{
			Eyebrow:  "Account",
			Heading:  "Hello, " + user.FirstName + ".",
			Body:     body,
			Footnote: "This email is for " + user.FullName() + ". If you did not open an account, you can ignore this email.",
		})
		if err != nil {
			return User{}, middleware.Internal("internal_error", "internal server error")
		}
		if err := s.send(ctx, mail.Message{
			ToEmail: user.Email, ToName: user.FullName(),
			Subject: "Your VirungaPay account",
			Text:    body,
			HTML:    rendered,
		}); err != nil {
			return User{}, err
		}
		return user, nil
	}
	if err := s.sendVerification(ctx, user); err != nil {
		return User{}, err
	}
	return user, nil
}

// VerifyEmail marks the account verified. The raw token is not stored.
func (s *Service) VerifyEmail(ctx context.Context, raw string) error {
	token, err := s.users.EmailToken(ctx, hashToken(raw))
	if err != nil {
		return middleware.Unprocessable("invalid_token", "verification link is invalid or expired")
	}
	if token.UsedAt != nil {
		user, lookupErr := s.users.ByID(ctx, token.UserID)
		if lookupErr != nil || user.EmailVerifiedAt == nil {
			return middleware.Unprocessable("invalid_token", "verification link is invalid or expired")
		}
		return nil
	}
	if s.now().After(token.ExpiresAt) {
		return middleware.Unprocessable("invalid_token", "verification link is invalid or expired")
	}
	now := s.now()
	if err := s.users.SetEmailVerified(ctx, token.UserID, now); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	if err := s.users.UseEmailToken(ctx, token.ID, now); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}

// EnsureStaff creates the superadmin used for /admin/v1 when that email does not exist yet.
// Empty email and password skip the step. An existing staff account is left unchanged.
func (s *Service) EnsureStaff(ctx context.Context, name, email, password string) error {
	email = normalizeEmail(email)
	if email == "" && password == "" {
		return nil
	}
	if email == "" || len(password) < passwordMinLen {
		return middleware.Invalid("invalid_request", "ADMIN_EMAIL and a password of at least 8 characters are required together")
	}
	existing, err := s.users.ByEmail(ctx, email)
	if err == nil {
		if !isStaff(existing.Role) {
			return middleware.Conflict("email_taken", "ADMIN_EMAIL belongs to a non-staff account")
		}
		return nil
	}
	if !errors.Is(err, errNotFound) {
		return middleware.Internal("internal_error", "internal server error")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	now := s.now()
	if strings.TrimSpace(name) == "" {
		name = "VirungaPay Admin"
	}
	first, last := splitStoredName(name)
	user := User{
		ID:              uuid.NewString(),
		Role:            RoleVASuperadmin,
		FirstName:       first,
		LastName:        last,
		Email:           email,
		PasswordHash:    string(hash),
		Status:          StatusActive,
		KYCStatus:       "none",
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, errDuplicate) {
			return nil
		}
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}

// Login issues a 24-hour HS256 token for the requested audience.
func (s *Service) Login(ctx context.Context, in Login, audience string) (Session, error) {
	denied := middleware.Unauthorized("invalid_credentials", "email or password is incorrect")
	user, err := s.users.ByEmail(ctx, normalizeEmail(in.Email))
	if err != nil {
		return Session{}, denied
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)) != nil {
		return Session{}, denied
	}
	if user.Status != StatusActive {
		return Session{}, denied
	}
	if !allowed(user.Role, audience) {
		return Session{}, denied
	}
	if needsVerification(user.Role) && user.EmailVerifiedAt == nil {
		return Session{}, middleware.Forbidden("email_unverified", "verify your email before signing in")
	}
	orgID, err := s.members.OrganisationID(ctx, user.ID)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	signed, err := s.sign(user, orgID)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	return Session{Token: signed, User: user.public(orgID)}, nil
}

// RequestLoginCode emails a 10-minute code for the phone app.
// An unknown email, a staff account, and an inactive account all get the same success and no email.
func (s *Service) RequestLoginCode(ctx context.Context, email string) error {
	user, err := s.users.ByEmail(ctx, normalizeEmail(email))
	if err != nil || user.Status != StatusActive || !allowed(user.Role, AudiencePWA) {
		return nil
	}
	code, err := newLoginCode()
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	row := LoginCode{
		ID: uuid.NewString(), UserID: user.ID, CodeHash: hashToken(code), ExpiresAt: s.now().Add(loginCodeTTL),
	}
	if err := s.users.SaveLoginCode(ctx, row); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	text := "Your VirungaPay code is " + code + ". It expires in 10 minutes."
	rendered, err := mail.RenderStandard(mail.Standard{
		Eyebrow:  "Sign in",
		Heading:  "Hello, " + user.FirstName + ".",
		Body:     "Enter this code in the VirungaPay phone app. It expires in 10 minutes.",
		Code:     code,
		Footnote: "This code is for " + user.FullName() + ". If you did not try to sign in, you can ignore this email.",
	})
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return s.send(ctx, mail.Message{
		ToEmail: user.Email, ToName: user.FullName(),
		Subject: "Your VirungaPay code",
		Text:    text,
		HTML:    rendered,
	})
}

// ConfirmLoginCode signs in a phone-app user and marks the email verified.
func (s *Service) ConfirmLoginCode(ctx context.Context, email, code string) (Session, error) {
	denied := middleware.Unauthorized("invalid_code", "that code is incorrect or expired")
	user, err := s.users.ByEmail(ctx, normalizeEmail(email))
	if err != nil || user.Status != StatusActive || !allowed(user.Role, AudiencePWA) {
		return Session{}, denied
	}
	saved, err := s.users.LatestLoginCode(ctx, user.ID)
	if err != nil || saved.UsedAt != nil || s.now().After(saved.ExpiresAt) {
		return Session{}, denied
	}
	got := hashToken(strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(got), []byte(saved.CodeHash)) != 1 {
		return Session{}, denied
	}
	now := s.now()
	if err := s.users.UseLoginCode(ctx, saved.ID, now); err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	if user.EmailVerifiedAt == nil {
		if err := s.users.SetEmailVerified(ctx, user.ID, now); err != nil {
			return Session{}, middleware.Internal("internal_error", "internal server error")
		}
		user.EmailVerifiedAt = &now
	}
	orgID, err := s.members.OrganisationID(ctx, user.ID)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	signed, err := s.sign(user, orgID)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	return Session{Token: signed, User: user.public(orgID)}, nil
}

// ForgotPassword emails a one-hour reset link for an active verified user.
// The HTTP caller always receives the same success, including when the email is unknown.
func (s *Service) ForgotPassword(ctx context.Context, email, audience string) error {
	user, err := s.users.ByEmail(ctx, normalizeEmail(email))
	if err != nil || user.Status != StatusActive || user.EmailVerifiedAt == nil || !allowed(user.Role, audience) {
		return nil
	}
	raw, hash, err := newToken()
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	token := ResetToken{ID: uuid.NewString(), UserID: user.ID, TokenHash: hash, ExpiresAt: s.now().Add(resetTTL)}
	if err := s.users.SaveResetToken(ctx, token); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	link := s.public + "/reset-password?token=" + raw
	return s.send(ctx, mail.Message{
		ToEmail: user.Email,
		ToName:  user.FullName(),
		Subject: "Reset your VirungaPay password",
		Text:    "Reset your password: " + link + "\nThis link expires in one hour.",
		HTML:    "<p>Reset your password: <a href=\"" + link + "\">" + link + "</a></p>",
	})
}

// ResetPassword sets a new password and invalidates the token.
func (s *Service) ResetPassword(ctx context.Context, in ResetPassword, audience string) error {
	if len(in.Password) < passwordMinLen {
		return middleware.Invalid("invalid_request", "password must be at least 8 characters")
	}
	token, err := s.users.ResetToken(ctx, hashToken(in.Token))
	if err != nil || token.UsedAt != nil || s.now().After(token.ExpiresAt) {
		return middleware.Unprocessable("invalid_token", "reset link is invalid or expired")
	}
	user, err := s.users.ByID(ctx, token.UserID)
	if err != nil || !allowed(user.Role, audience) || user.Status != StatusActive {
		return middleware.Unprocessable("invalid_token", "reset link is invalid or expired")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	now := s.now()
	if err := s.users.SetPassword(ctx, user.ID, string(hash), now); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	if err := s.users.UseResetToken(ctx, token.ID, now); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	return nil
}

// CreateInvitedUser inserts a guide or driver who cannot sign in until they accept the invite.
// The returned secret is the invite token. Callers email it and must not return it over HTTP.
func (s *Service) CreateInvitedUser(ctx context.Context, name, email, phone, role string) (User, string, string, error) {
	if role != RoleGuide && role != RoleDriver && role != RoleForexBureauOwner {
		return User{}, "", "", middleware.Invalid("invalid_role", "invite role must be guide, driver, or forex bureau owner")
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" {
		return User{}, "", "", middleware.Invalid("invalid_request", "name and email are required")
	}
	placeholder, err := randomSecret()
	if err != nil {
		return User{}, "", "", middleware.Internal("internal_error", "internal server error")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(placeholder), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", "", middleware.Internal("internal_error", "internal server error")
	}
	raw, tokenHash, err := newToken()
	if err != nil {
		return User{}, "", "", middleware.Internal("internal_error", "internal server error")
	}
	now := s.now()
	first, last := splitStoredName(name)
	user := User{
		ID:           uuid.NewString(),
		Role:         role,
		FirstName:    first,
		LastName:     last,
		Email:        normalizeEmail(email),
		Phone:        strings.TrimSpace(phone),
		PasswordHash: string(hash),
		Status:       StatusInvited,
		KYCStatus:    "none",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, errDuplicate) {
			return User{}, "", "", middleware.Conflict("email_taken", "an account with this email already exists")
		}
		return User{}, "", "", middleware.Internal("internal_error", "internal server error")
	}
	return user, raw, tokenHash, nil
}

// AcceptInvite sets the password, verifies the email, and activates the invited user.
func (s *Service) AcceptInvite(ctx context.Context, in AcceptInvite) (Session, error) {
	if len(in.Password) < passwordMinLen {
		return Session{}, middleware.Invalid("invalid_request", "password must be at least 8 characters")
	}
	invite, err := s.invites.FindByTokenHash(ctx, hashToken(in.Token))
	if err != nil || invite.AcceptedAt != nil || s.now().After(invite.ExpiresAt) {
		return Session{}, middleware.Unprocessable("invalid_token", "invite link is invalid or expired")
	}
	user, err := s.users.ByID(ctx, invite.UserID)
	if err != nil || user.Status != StatusInvited {
		return Session{}, middleware.Unprocessable("invalid_token", "invite link is invalid or expired")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	now := s.now()
	if err := s.users.SetPassword(ctx, user.ID, string(hash), now); err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.users.SetEmailVerified(ctx, user.ID, now); err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.users.SetStatus(ctx, user.ID, StatusActive, now); err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	if err := s.invites.MarkAccepted(ctx, invite.ID, now); err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	user.Status = StatusActive
	user.EmailVerifiedAt = &now
	signed, err := s.sign(user, invite.OrganisationID)
	if err != nil {
		return Session{}, middleware.Internal("internal_error", "internal server error")
	}
	return Session{Token: signed, User: user.public(invite.OrganisationID)}, nil
}

// UpdatePhone saves a phone number on a tourist profile.
func (s *Service) UpdatePhone(ctx context.Context, userID, phone string) (PublicUser, error) {
	phone = strings.TrimSpace(phone)
	if !phonePattern.MatchString(phone) {
		return PublicUser{}, middleware.Invalid("invalid_phone", "phone must be an international number such as +256700000000")
	}
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return PublicUser{}, middleware.Unauthorized("invalid_credentials", "sign in again")
	}
	if user.Role != RoleTourist {
		return PublicUser{}, middleware.Forbidden("forbidden", "only a tourist can add a phone number here")
	}
	now := s.now()
	if err := s.users.SetPhone(ctx, user.ID, phone, now); err != nil {
		if errors.Is(err, errDuplicate) {
			return PublicUser{}, middleware.Conflict("phone_taken", "an account with this phone number already exists")
		}
		return PublicUser{}, middleware.Internal("internal_error", "internal server error")
	}
	user.Phone = phone
	user.UpdatedAt = now
	orgID, err := s.members.OrganisationID(ctx, user.ID)
	if err != nil {
		return PublicUser{}, middleware.Internal("internal_error", "internal server error")
	}
	return user.public(orgID), nil
}

// Me loads the current person.
func (s *Service) Me(ctx context.Context, userID string) (PublicUser, error) {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return PublicUser{}, middleware.Unauthorized("invalid_credentials", "sign in again")
	}
	orgID, err := s.members.OrganisationID(ctx, user.ID)
	if err != nil {
		return PublicUser{}, middleware.Internal("internal_error", "internal server error")
	}
	return user.public(orgID), nil
}

// ParseToken validates an HS256 session token.
func (s *Service) ParseToken(raw string) (Claims, error) {
	claims := Claims{}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, middleware.Unauthorized("invalid_credentials", "sign in again")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid || claims.Subject == "" {
		return Claims{}, middleware.Unauthorized("invalid_credentials", "sign in again")
	}
	return claims, nil
}

// Claims are the session fields.
type Claims struct {
	jwt.RegisteredClaims
	Role           string `json:"role"`
	OrganisationID string `json:"organisation_id,omitempty"`
}

func (s *Service) sendVerification(ctx context.Context, user User) error {
	raw, hash, err := newToken()
	if err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	token := EmailToken{ID: uuid.NewString(), UserID: user.ID, TokenHash: hash, ExpiresAt: s.now().Add(verifyTTL)}
	if err := s.users.SaveEmailToken(ctx, token); err != nil {
		return middleware.Internal("internal_error", "internal server error")
	}
	link := s.public + "/verify-email?token=" + raw
	subject := "Verify your VirungaPay email"
	text := "Verify your email: " + link
	body := "<p>Verify your email: <a href=\"" + link + "\">" + link + "</a></p>"
	if user.Role == RoleOperatorOwner {
		subject = "Welcome to VirungaPay"
		text = "Welcome to VirungaPay, " + user.FullName() + ".\n\nVerify your email to open your operator account: " + link
		rendered, err := mail.RenderWelcome(mail.Welcome{
			FirstName: user.FirstName,
			FullName:  user.FullName(),
			VerifyURL: link,
		})
		if err != nil {
			return middleware.Internal("internal_error", "internal server error")
		}
		body = rendered
	}
	return s.send(ctx, mail.Message{
		ToEmail: user.Email,
		ToName:  user.FullName(),
		Subject: subject,
		Text:    text,
		HTML:    body,
	})
}

func (s *Service) send(ctx context.Context, message mail.Message) error {
	if err := s.mail.Send(ctx, message); err != nil {
		return middleware.Unavailable("mail_unavailable", "email could not be sent")
	}
	return nil
}

func (s *Service) sign(user User, organisationID string) (string, error) {
	now := s.now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
		Role:           user.Role,
		OrganisationID: organisationID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func validateSignup(in Signup) error {
	if strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" || strings.TrimSpace(in.Email) == "" {
		return middleware.Invalid("invalid_request", "first name, last name, and email are required")
	}
	if len(in.Password) < passwordMinLen {
		return middleware.Invalid("invalid_request", "password must be at least 8 characters")
	}
	return nil
}

func personName(first, last string) (string, string) {
	return capitalizeName(first), capitalizeName(last)
}

func splitStoredName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return capitalizeName(parts[0]), ""
	}
	return capitalizeName(parts[0]), capitalizeName(strings.Join(parts[1:], " "))
}

func capitalizeName(value string) string {
	fields := strings.Fields(value)
	for i, field := range fields {
		r, size := utf8.DecodeRuneInString(field)
		if r == utf8.RuneError && size == 1 {
			continue
		}
		fields[i] = string(unicode.ToUpper(r)) + strings.ToLower(field[size:])
	}
	return strings.Join(fields, " ")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func needsVerification(role string) bool {
	return role == RoleTourist || role == RoleOperatorOwner || role == RoleForexBureauOwner
}

func allowed(role, audience string) bool {
	switch audience {
	case AudienceAdmin:
		return isStaff(role)
	default:
		switch role {
		case RoleTourist, RoleOperatorOwner, RoleGuide, RoleDriver, RoleForexBureauOwner:
			return true
		default:
			return false
		}
	}
}

func isStaff(role string) bool {
	switch role {
	case RoleVAOps, RoleVAFinance, RoleVASuperadmin, RoleAutoAdmin, RoleUHOAAdmin:
		return true
	default:
		return false
	}
}

func newLoginCode() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf) % 1000000
	return sprintfCode(n), nil
}

func sprintfCode(n uint32) string {
	return string([]byte{
		byte('0' + (n/100000)%10),
		byte('0' + (n/10000)%10),
		byte('0' + (n/1000)%10),
		byte('0' + (n/100)%10),
		byte('0' + (n/10)%10),
		byte('0' + n%10),
	})
}

func newToken() (raw, hash string, err error) {
	raw, err = randomSecret()
	if err != nil {
		return "", "", err
	}
	return raw, hashToken(raw), nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// HashToken is the stored form of an invite or email token.
func HashToken(raw string) string { return hashToken(raw) }
