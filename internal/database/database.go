package database

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"virungapay-be/internal/config"
)

// Service is the process-wide database handle.
type Service interface {
	Health() map[string]string
	Close() error
	DB() *gorm.DB
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type service struct {
	gdb *gorm.DB
}

// New opens a GORM Postgres connection and applies the pool settings from cfg.
func New(cfg config.DatabaseConfig) (Service, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database url is required")
	}
	gdb, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("sql pool: %w", err)
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen < 1 {
		maxOpen = 25
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle < 0 {
		maxIdle = 5
	}
	lifetime := cfg.ConnMaxLifetime
	if lifetime <= 0 {
		lifetime = 30 * time.Minute
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(lifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &service{gdb: gdb}, nil
}

// Health pings Postgres and returns pool statistics. A failed ping does not exit the process.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stats := map[string]string{}
	sqlDB, err := s.gdb.DB()
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"

	dbStats := sqlDB.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	if dbStats.OpenConnections > 40 {
		stats["message"] = "The database is experiencing heavy load."
	}
	if dbStats.WaitCount > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 && dbStats.OpenConnections > 0 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}
	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 && dbStats.OpenConnections > 0 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}
	return stats
}

// Close releases the underlying connection pool.
func (s *service) Close() error {
	sqlDB, err := s.gdb.DB()
	if err != nil {
		return err
	}
	if logger := zerolog.DefaultContextLogger; logger != nil {
		logger.Info().Msg("disconnected from database")
	} else {
		log.Printf("disconnected from database")
	}
	return sqlDB.Close()
}

// DB returns the GORM handle. Schema changes belong in migrations/, not AutoMigrate.
func (s *service) DB() *gorm.DB {
	return s.gdb
}

// WithTx runs fn inside a single database transaction.
func (s *service) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.gdb.WithContext(ctx).Transaction(fn)
}
