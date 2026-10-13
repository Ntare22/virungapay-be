package database

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrateUp applies every pending SQL migration in dir.
func MigrateUp(databaseURL, dir string) error {
	return migrateDirection(databaseURL, dir, true)
}

// MigrateDown rolls back one SQL migration in dir.
func MigrateDown(databaseURL, dir string) error {
	return migrateDirection(databaseURL, dir, false)
}

func migrateDirection(databaseURL, dir string, up bool) error {
	m, err := newMigrate(databaseURL, dir)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if up {
		err = m.Up()
	} else {
		err = m.Steps(-1)
	}
	if errors.Is(err, migrate.ErrNoChange) || errors.Is(err, migrate.ErrNilVersion) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func newMigrate(databaseURL, dir string) (*migrate.Migrate, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("migrations path: %w", err)
	}
	source := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	m, err := migrate.New(source.String(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return m, nil
}
