package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Repository reads and writes users and one-time tokens. It has no business rules.
type Repository struct {
	db *gorm.DB
}

// NewRepository uses the application GORM handle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, user User) error {
	row := UserRecord{
		ID:              user.ID,
		Role:            user.Role,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		PasswordHash:    user.PasswordHash,
		Status:          user.Status,
		KYCStatus:       user.KYCStatus,
		EmailVerifiedAt: user.EmailVerifiedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
	if user.Phone != "" {
		row.Phone = &user.Phone
	}
	return mapDuplicate(r.db.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) ByEmail(ctx context.Context, email string) (User, error) {
	var row UserRecord
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&row).Error
	return fromRow(row), mapNotFound(err)
}

func (r *Repository) ByID(ctx context.Context, id string) (User, error) {
	var row UserRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	return fromRow(row), mapNotFound(err)
}

func (r *Repository) SetPassword(ctx context.Context, id, hash string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&UserRecord{}).Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash, "updated_at": at}).Error
}

func (r *Repository) SetEmailVerified(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&UserRecord{}).Where("id = ?", id).
		Updates(map[string]any{"email_verified_at": at, "updated_at": at}).Error
}

func (r *Repository) SetStatus(ctx context.Context, id, status string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&UserRecord{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": at}).Error
}

func (r *Repository) SetPhone(ctx context.Context, id, phone string, at time.Time) error {
	return mapDuplicate(r.db.WithContext(ctx).Model(&UserRecord{}).Where("id = ?", id).
		Updates(map[string]any{"phone": phone, "updated_at": at}).Error)
}

func (r *Repository) SaveEmailToken(ctx context.Context, token EmailToken) error {
	row := emailVerificationRow{
		ID: token.ID, UserID: token.UserID, TokenHash: token.TokenHash, ExpiresAt: token.ExpiresAt, CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) EmailToken(ctx context.Context, hash string) (EmailToken, error) {
	var row emailVerificationRow
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	return EmailToken{ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt}, mapNotFound(err)
}

func (r *Repository) UseEmailToken(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&emailVerificationRow{}).Where("id = ?", id).Update("used_at", at).Error
}

func (r *Repository) SaveResetToken(ctx context.Context, token ResetToken) error {
	row := passwordResetRow{
		ID: token.ID, UserID: token.UserID, TokenHash: token.TokenHash, ExpiresAt: token.ExpiresAt, CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ResetToken(ctx context.Context, hash string) (ResetToken, error) {
	var row passwordResetRow
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	return ResetToken{ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt}, mapNotFound(err)
}

func (r *Repository) UseResetToken(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&passwordResetRow{}).Where("id = ?", id).Update("used_at", at).Error
}

func (r *Repository) SaveLoginCode(ctx context.Context, code LoginCode) error {
	row := loginCodeRow{
		ID: code.ID, UserID: code.UserID, CodeHash: code.CodeHash, ExpiresAt: code.ExpiresAt, CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) LatestLoginCode(ctx context.Context, userID string) (LoginCode, error) {
	var row loginCodeRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").First(&row).Error
	return LoginCode{ID: row.ID, UserID: row.UserID, CodeHash: row.CodeHash, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt}, mapNotFound(err)
}

func (r *Repository) UseLoginCode(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&loginCodeRow{}).Where("id = ?", id).Update("used_at", at).Error
}

type emailVerificationRow struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	UserID    string `gorm:"type:uuid;not null"`
	TokenHash string `gorm:"not null"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (emailVerificationRow) TableName() string { return "email_verifications" }

type passwordResetRow struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	UserID    string `gorm:"type:uuid;not null"`
	TokenHash string `gorm:"not null"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (passwordResetRow) TableName() string { return "password_resets" }

type loginCodeRow struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	UserID    string `gorm:"type:uuid;not null"`
	CodeHash  string `gorm:"not null"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (loginCodeRow) TableName() string { return "login_codes" }

func fromRow(row UserRecord) User {
	user := User{
		ID:              row.ID,
		Role:            row.Role,
		FirstName:       row.FirstName,
		LastName:        row.LastName,
		Email:           row.Email,
		PasswordHash:    row.PasswordHash,
		Status:          row.Status,
		KYCStatus:       row.KYCStatus,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.Phone != nil {
		user.Phone = *row.Phone
	}
	return user
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errNotFound
	}
	return err
}

func mapDuplicate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errDuplicate
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errDuplicate
	}
	return err
}

var (
	errNotFound  = errors.New("not found")
	errDuplicate = errors.New("duplicate")
)
