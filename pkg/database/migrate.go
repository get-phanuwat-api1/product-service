package database

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrator wraps golang-migrate for a PostgreSQL database.
type Migrator struct {
	m *migrate.Migrate
}

// NewMigrator creates a migrator reading *.sql files from fsys (root directory).
func NewMigrator(fsys fs.FS, dsn string) (*Migrator, error) {
	src, err := iofs.New(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("open migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toMigrateURL(dsn))
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}
	return &Migrator{m: m}, nil
}

// Up applies all pending migrations.
func (mg *Migrator) Up() error { return ignoreNoChange(mg.m.Up()) }

// Down rolls back the given number of steps.
func (mg *Migrator) Down(steps int) error { return ignoreNoChange(mg.m.Steps(-steps)) }

// Force sets the version without running migrations (used to fix a dirty state).
func (mg *Migrator) Force(version int) error { return mg.m.Force(version) }

// Version returns the current version and whether it is dirty.
func (mg *Migrator) Version() (uint, bool, error) {
	v, dirty, err := mg.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

// Close releases the source and database connections.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	return errors.Join(srcErr, dbErr)
}

// MigrateUp is a convenience helper that applies all pending migrations.
func MigrateUp(fsys fs.FS, dsn string) error {
	mg, err := NewMigrator(fsys, dsn)
	if err != nil {
		return err
	}
	defer mg.Close()
	return mg.Up()
}

func ignoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

// toMigrateURL converts postgres:// DSNs to the pgx5:// scheme used by golang-migrate.
// pool_* params are pgxpool-only and are stripped.
func toMigrateURL(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, prefix) {
			dsn = "pgx5://" + strings.TrimPrefix(dsn, prefix)
			break
		}
	}
	base, query, found := strings.Cut(dsn, "?")
	if !found {
		return dsn
	}
	var kept []string
	for _, kv := range strings.Split(query, "&") {
		if kv != "" && !strings.HasPrefix(kv, "pool_") {
			kept = append(kept, kv)
		}
	}
	if len(kept) == 0 {
		return base
	}
	return base + "?" + strings.Join(kept, "&")
}
