package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/virungapay_test?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173, http://localhost:5174")
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("DB_MAX_IDLE_CONNS", "2")
	t.Setenv("DB_CONN_MAX_LIFETIME", "1m")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 9090, cfg.Port)
	require.Equal(t, "test", cfg.AppEnv)
	require.Equal(t, []string{"http://localhost:5173", "http://localhost:5174"}, cfg.CORSOrigins)
	require.Equal(t, 10, cfg.Database.MaxOpenConns)
	require.Equal(t, 2, cfg.Database.MaxIdleConns)
	require.Equal(t, time.Minute, cfg.Database.ConnMaxLifetime)
	require.False(t, cfg.ExposeDocs() && cfg.AppEnv == "production")
	require.True(t, cfg.ExposeDocs())
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173")

	_, err := Load()
	require.ErrorContains(t, err, "DATABASE_URL")
}
