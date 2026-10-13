package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"

	"virungapay-be/internal/config"
)

var testDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()
	dbName := "database"
	dbUser := "user"
	dbPwd := "password"

	container, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		panic(fmt.Errorf("could not start postgres container: %w", err))
	}

	host, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		panic(err)
	}
	testDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPwd, host, port.Port(), dbName)

	code := m.Run()
	if err := container.Terminate(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "could not teardown postgres container: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func openTestDB(t *testing.T) Service {
	t.Helper()
	srv, err := New(config.DatabaseConfig{
		URL:             testDSN,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})
	return srv
}

func TestNew(t *testing.T) {
	srv := openTestDB(t)
	require.NotNil(t, srv.DB())
}

func TestHealth(t *testing.T) {
	stats := openTestDB(t).Health()
	require.Equal(t, "up", stats["status"])
	_, hasError := stats["error"]
	require.False(t, hasError)
	require.Equal(t, "It's healthy", stats["message"])
}

func TestWithTx(t *testing.T) {
	srv := openTestDB(t)
	err := srv.WithTx(context.Background(), func(tx *gorm.DB) error {
		return tx.Exec("SELECT 1").Error
	})
	require.NoError(t, err)
}
