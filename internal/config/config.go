// Package config loads process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the process configuration. Database access uses DATABASE_URL only.
type Config struct {
	Port        int
	AppEnv      string
	JWTSecret   string
	CORSOrigins []string
	Database    DatabaseConfig
	Mail        MailConfig
	Staff       StaffConfig
}

// StaffConfig is the local superadmin created at startup when both fields are set.
type StaffConfig struct {
	Name     string
	Email    string
	Password string
}

// MailConfig is Mailjet and the public app origin used in emailed links.
type MailConfig struct {
	APIKey    string
	SecretKey string
	From      string
	PublicURL string
}

// DatabaseConfig is the Postgres pool configuration.
type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Load reads .env without overriding variables that are already set.
func Load() (Config, error) {
	_ = godotenv.Load()

	port, err := getenvInt("PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	maxOpen, err := getenvInt("DB_MAX_OPEN_CONNS", 25)
	if err != nil {
		return Config{}, err
	}
	maxIdle, err := getenvInt("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return Config{}, err
	}
	lifetime, err := getenvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(jwtSecret) == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}
	origins := splitCSV(os.Getenv("CORS_ORIGINS"))
	if len(origins) == 0 {
		return Config{}, errors.New("CORS_ORIGINS is required")
	}
	if maxOpen < 1 {
		return Config{}, errors.New("DB_MAX_OPEN_CONNS must be >= 1")
	}
	if maxIdle < 0 || maxIdle > maxOpen {
		return Config{}, errors.New("DB_MAX_IDLE_CONNS must be between 0 and DB_MAX_OPEN_CONNS")
	}

	return Config{
		Port:        port,
		AppEnv:      getenv("APP_ENV", "local"),
		JWTSecret:   jwtSecret,
		CORSOrigins: origins,
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxOpenConns:    maxOpen,
			MaxIdleConns:    maxIdle,
			ConnMaxLifetime: lifetime,
		},
		Mail: MailConfig{
			APIKey:    os.Getenv("MAILJET_API_KEY"),
			SecretKey: os.Getenv("MAILJET_SECRET_KEY"),
			From:      getenv("MAIL_FROM", "no-reply@virungapay.demo"),
			PublicURL: strings.TrimRight(getenv("APP_PUBLIC_URL", "http://localhost:5173"), "/"),
		},
		Staff: StaffConfig{
			Name:     getenv("ADMIN_NAME", "VirungaPay Admin"),
			Email:    strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
			Password: os.Getenv("ADMIN_PASSWORD"),
		},
	}, nil
}

// ExposeDocs reports whether OpenAPI UI should be mounted.
func (c Config) ExposeDocs() bool {
	switch strings.ToLower(c.AppEnv) {
	case "prod", "production":
		return false
	default:
		return true
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
