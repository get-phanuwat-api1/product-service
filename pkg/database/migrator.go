package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationLockID is the PostgreSQL advisory lock key held while migrating,
// so concurrent instances never apply migrations at the same time.
const migrationLockID int64 = 7_362_100_001

const createHistoryTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version      BIGINT       PRIMARY KEY,
    description  VARCHAR(255) NOT NULL,
    checksum     CHAR(64)     NOT NULL,
    applied_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    execution_ms BIGINT       NOT NULL
)`

// AppliedMigration is one row of the schema_migrations history table.
type AppliedMigration struct {
	Version     int64
	Description string
	Checksum    string
	AppliedAt   time.Time
	ExecutionMs int64
}

// Migrator applies versioned SQL migrations and records one history row per migration
// in schema_migrations. Each migration and its history row commit in one transaction.
type Migrator struct {
	pool       *pgxpool.Pool
	migrations []Migration
}

// NewMigrator loads the migrations from fsys.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	migs, err := LoadMigrations(fsys)
	if err != nil {
		return nil, err
	}
	return &Migrator{pool: pool, migrations: migs}, nil
}

// MigrateUp is a convenience helper: apply every pending migration and return the ones applied.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]Migration, error) {
	m, err := NewMigrator(pool, fsys)
	if err != nil {
		return nil, err
	}
	return m.Up(ctx)
}

// Up applies all pending migrations in version order.
func (m *Migrator) Up(ctx context.Context) ([]Migration, error) {
	var applied []Migration
	err := m.withLock(ctx, func(conn *pgxpool.Conn) error {
		history, err := m.loadHistory(ctx, conn)
		if err != nil {
			return err
		}
		if err := m.verify(history); err != nil {
			return err
		}
		for _, mig := range m.migrations {
			if _, done := history[mig.Version]; done {
				continue
			}
			if err := m.apply(ctx, conn, mig); err != nil {
				return err
			}
			applied = append(applied, mig)
		}
		return nil
	})
	return applied, err
}

// Down rolls back the latest `steps` applied migrations, newest first.
func (m *Migrator) Down(ctx context.Context, steps int) ([]Migration, error) {
	if steps <= 0 {
		return nil, errors.New("steps must be > 0")
	}
	var reverted []Migration
	err := m.withLock(ctx, func(conn *pgxpool.Conn) error {
		history, err := m.loadHistory(ctx, conn)
		if err != nil {
			return err
		}
		for i := len(m.migrations) - 1; i >= 0 && len(reverted) < steps; i-- {
			mig := m.migrations[i]
			if _, done := history[mig.Version]; !done {
				continue
			}
			if err := m.revert(ctx, conn, mig); err != nil {
				return err
			}
			reverted = append(reverted, mig)
		}
		return nil
	})
	return reverted, err
}

// History returns the applied migrations, oldest first.
func (m *Migrator) History(ctx context.Context) ([]AppliedMigration, error) {
	var out []AppliedMigration
	err := m.withLock(ctx, func(conn *pgxpool.Conn) error {
		history, err := m.loadHistory(ctx, conn)
		if err != nil {
			return err
		}
		out = sortedHistory(history)
		return nil
	})
	return out, err
}

// Pending returns the migrations that are not applied yet.
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error) {
	history, err := m.History(ctx)
	if err != nil {
		return nil, err
	}
	done := map[int64]bool{}
	for _, h := range history {
		done[h.Version] = true
	}
	var pending []Migration
	for _, mig := range m.migrations {
		if !done[mig.Version] {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// --- internals -------------------------------------------------------------------

func (m *Migrator) withLock(ctx context.Context, fn func(conn *pgxpool.Conn) error) error {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	if err := m.ensureHistoryTable(ctx, conn); err != nil {
		return err
	}
	return fn(conn)
}

// ensureHistoryTable creates schema_migrations, converting a golang-migrate style
// table (version, dirty) into the history format if one is found.
func (m *Migrator) ensureHistoryTable(ctx context.Context, conn *pgxpool.Conn) error {
	var legacy bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'schema_migrations' AND column_name = 'dirty'
		)`).Scan(&legacy)
	if err != nil {
		return fmt.Errorf("inspect schema_migrations: %w", err)
	}
	if legacy {
		return m.convertLegacyTable(ctx, conn)
	}
	if _, err := conn.Exec(ctx, createHistoryTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) convertLegacyTable(ctx context.Context, conn *pgxpool.Conn) error {
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		var (
			version int64
			dirty   bool
		)
		err := tx.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read legacy schema_migrations: %w", err)
		}
		if dirty {
			return fmt.Errorf("legacy schema_migrations is dirty at version %d: fix the schema manually first", version)
		}
		if _, err := tx.Exec(ctx, `DROP TABLE schema_migrations`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, createHistoryTable); err != nil {
			return err
		}
		for _, mig := range m.migrations {
			if mig.Version > version {
				break
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO schema_migrations (version, description, checksum, execution_ms) VALUES ($1, $2, $3, 0)`,
				mig.Version, mig.Description, mig.Checksum); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *Migrator) loadHistory(ctx context.Context, conn *pgxpool.Conn) (map[int64]AppliedMigration, error) {
	rows, err := conn.Query(ctx,
		`SELECT version, description, checksum, applied_at, execution_ms FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	history := map[int64]AppliedMigration{}
	for rows.Next() {
		var h AppliedMigration
		if err := rows.Scan(&h.Version, &h.Description, &h.Checksum, &h.AppliedAt, &h.ExecutionMs); err != nil {
			return nil, err
		}
		history[h.Version] = h
	}
	return history, rows.Err()
}

// verify refuses to continue when an applied migration was edited or deleted.
func (m *Migrator) verify(history map[int64]AppliedMigration) error {
	files := map[int64]Migration{}
	for _, mig := range m.migrations {
		files[mig.Version] = mig
	}
	for _, h := range sortedHistory(history) {
		mig, ok := files[h.Version]
		if !ok {
			return fmt.Errorf("migration %d (%s) is applied but its file is missing", h.Version, h.Description)
		}
		if mig.Checksum != h.Checksum {
			return fmt.Errorf("migration %d (%s) was modified after it was applied: create a new migration instead", h.Version, h.Description)
		}
	}
	return nil
}

func (m *Migrator) apply(ctx context.Context, conn *pgxpool.Conn, mig Migration) error {
	start := time.Now()
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, mig.UpSQL); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, description, checksum, execution_ms) VALUES ($1, $2, $3, $4)`,
			mig.Version, mig.Description, mig.Checksum, time.Since(start).Milliseconds())
		return err
	})
	if err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	return nil
}

func (m *Migrator) revert(ctx context.Context, conn *pgxpool.Conn, mig Migration) error {
	if mig.DownSQL == "" {
		return fmt.Errorf("migration %d (%s) has no .down.sql file", mig.Version, mig.Description)
	}
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, mig.DownSQL); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mig.Version)
		return err
	})
	if err != nil {
		return fmt.Errorf("revert migration %d (%s): %w", mig.Version, mig.Description, err)
	}
	return nil
}

func sortedHistory(history map[int64]AppliedMigration) []AppliedMigration {
	out := make([]AppliedMigration, 0, len(history))
	for _, h := range history {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}
