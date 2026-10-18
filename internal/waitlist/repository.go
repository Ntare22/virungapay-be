package waitlist

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type row struct {
	Email     string `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (row) TableName() string { return "waitlist" }

// Repository writes waitlist rows.
type Repository struct {
	db *gorm.DB
}

// NewRepository uses the application GORM handle.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save inserts the email. A duplicate address is left as it is.
func (r *Repository) Save(ctx context.Context, email string, at time.Time) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row{Email: email, CreatedAt: at}).Error
}
