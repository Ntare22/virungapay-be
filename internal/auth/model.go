// Package auth registers tourists, signs people in, and verifies email.
package auth

import "time"

// UserRecord is the users table.
type UserRecord struct {
	ID              string `gorm:"type:uuid;primaryKey"`
	Role            string `gorm:"type:user_role;not null"`
	FirstName       string `gorm:"column:first_name;not null"`
	LastName        string `gorm:"column:last_name;not null"`
	Email           string `gorm:"not null"`
	Phone           *string
	PasswordHash    string     `gorm:"not null"`
	Status          string     `gorm:"type:user_status;not null"`
	KYCStatus       string     `gorm:"column:kyc_status;type:kyc_status;not null"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at"`
	CreatedAt       time.Time  `gorm:"not null"`
	UpdatedAt       time.Time  `gorm:"not null"`
}

func (UserRecord) TableName() string { return "users" }

const (
	RoleTourist          = "tourist"
	RoleOperatorOwner    = "operator_owner"
	RoleGuide            = "guide"
	RoleDriver           = "driver"
	RoleForexBureauOwner = "forex_bureau_owner"
	RoleOperatorStaff    = "operator_staff"
	RoleAutoAdmin        = "auto_admin"
	RoleUHOAAdmin        = "uhoa_admin"
	RoleVAOps            = "va_ops"
	RoleVAFinance        = "va_finance"
	RoleVASuperadmin     = "va_superadmin"

	StatusInvited   = "invited"
	StatusActive    = "active"
	StatusSuspended = "suspended"

	AudiencePWA   = "pwa"
	AudienceAdmin = "admin"
)

// Signup is a public registration. The role is chosen by the endpoint, not the body.
type Signup struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Password  string
}

// Login is an email and password.
type Login struct {
	Email    string
	Password string
}

// Token is a one-time secret from an email link.
type Token struct {
	Token string
}

// ResetPassword sets a new password with a reset token.
type ResetPassword struct {
	Token    string
	Password string
}

// AcceptInvite sets a password for an invited guide or driver.
type AcceptInvite struct {
	Token    string
	Password string
}

// User is the person record used by this feature.
type User struct {
	ID              string
	Role            string
	FirstName       string
	LastName        string
	Email           string
	Phone           string
	PasswordHash    string
	Status          string
	KYCStatus       string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PublicUser is safe to return from HTTP.
type PublicUser struct {
	ID             string `json:"id"`
	Role           string `json:"role"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone,omitempty"`
	Status         string `json:"status"`
	EmailVerified  bool   `json:"email_verified"`
	OrganisationID string `json:"organisation_id,omitempty"`
}

// RegisterResponse is a signup acknowledgement. It has no session token.
type RegisterResponse struct {
	User PublicUser `json:"user"`
}

// Session is a login result.
type Session struct {
	Token string     `json:"token"`
	User  PublicUser `json:"user"`
}

// MessageResponse is a generic acknowledgement that never includes a token.
type MessageResponse struct {
	Message string `json:"message"`
}

// TouristSignupRequest is the JSON body for tourist signup. Phone and password are not collected here.
type TouristSignupRequest struct {
	FirstName string `json:"first_name" binding:"required" example:"Amelie"`
	LastName  string `json:"last_name" binding:"required" example:"Laurent"`
	Email     string `json:"email" binding:"required,email" example:"amelie@example.com"`
}

// SignupRequest is the JSON body for operator signup.
type SignupRequest struct {
	FirstName string `json:"first_name" binding:"required" example:"Amina"`
	LastName  string `json:"last_name" binding:"required" example:"Kato"`
	Email     string `json:"email" binding:"required,email" example:"amina@example.com"`
	Phone     string `json:"phone" example:"+256700000000"`
	Password  string `json:"password" binding:"required,min=8" example:"correct horse"`
}

// ProfileRequest sets the signed-in tourist's phone number.
type ProfileRequest struct {
	Phone string `json:"phone" binding:"required" example:"+256700000000"`
}

// LoginRequest is the JSON body for login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"amelie@example.com"`
	Password string `json:"password" binding:"required" example:"correct horse"`
}

// TokenRequest is the JSON body for verify-email.
type TokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// ResetRequest is the JSON body for reset-password.
type ResetRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// ForgotRequest is the JSON body for forgot-password.
type ForgotRequest struct {
	Email string `json:"email" binding:"required,email" example:"amelie@example.com"`
}

// LoginCodeRequest asks for a phone-app sign-in code.
type LoginCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"amelie@example.com"`
}

// ConfirmLoginCodeRequest is the email and the code from that email.
type ConfirmLoginCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"amelie@example.com"`
	Code  string `json:"code" binding:"required,len=6" example:"123456"`
}

// LoginCode is a stored phone sign-in code. Only the hash is persisted.
type LoginCode struct {
	ID        string
	UserID    string
	CodeHash  string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// AcceptRequest is the JSON body for accepting an invite.
type AcceptRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// EmailToken is a stored verification secret. Only the hash is persisted.
type EmailToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// ResetToken is a stored password-reset secret. Only the hash is persisted.
type ResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// PendingInvite is an unused company invite.
type PendingInvite struct {
	ID             string
	OrganisationID string
	UserID         string
	Role           string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
}

func (u User) FullName() string {
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

func (u User) public(organisationID string) PublicUser {
	return PublicUser{
		ID:             u.ID,
		Role:           u.Role,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Name:           u.FullName(),
		Email:          u.Email,
		Phone:          u.Phone,
		Status:         u.Status,
		EmailVerified:  u.EmailVerifiedAt != nil,
		OrganisationID: organisationID,
	}
}
