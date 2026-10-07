// Package testutil contains helpers shared by repository and component tests.
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/phanuwat/product-service/migrations"
	"github.com/phanuwat/product-service/pkg/database"
)

// StartPostgres returns a migrated pgx pool for tests.
//
// If TEST_DATABASE_URL is set it is used directly; otherwise a disposable
// PostgreSQL container is started with testcontainers (Docker required).
// The returned cleanup func must be called when done.
func StartPostgres(ctx context.Context) (*pgxpool.Pool, func(), error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	var container *tcpostgres.PostgresContainer

	if dsn == "" {
		var err error
		container, err = tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("product_test"),
			tcpostgres.WithUsername("postgres"),
			tcpostgres.WithPassword("postgres"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
			),
		)
		if err != nil {
			return nil, nil, err
		}
		dsn, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			_ = container.Terminate(ctx)
			return nil, nil, err
		}
	}

	terminate := func() {
		if container != nil {
			_ = container.Terminate(context.Background())
		}
	}

	if err := database.MigrateUp(migrations.FS, dsn); err != nil {
		terminate()
		return nil, nil, err
	}
	pool, err := database.NewPostgresPool(ctx, dsn)
	if err != nil {
		terminate()
		return nil, nil, err
	}

	return pool, func() { pool.Close(); terminate() }, nil
}

// TruncateProducts clears the products table between tests.
func TruncateProducts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE products RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate products: %v", err)
	}
}
