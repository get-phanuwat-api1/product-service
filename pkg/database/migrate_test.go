package database_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/migrations"
	"github.com/get-phanuwat-api1/product-service/pkg/database"
	"github.com/get-phanuwat-api1/product-service/test/testutil"
)

// TestMigrateUp_ConcurrentStartsAreSafe simulates several API instances starting at once
// with auto-migrate enabled: all must succeed and end on the same version.
func TestMigrateUp_ConcurrentStartsAreSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Docker")
	}
	if os.Getenv("TEST_DATABASE_URL") != "" {
		t.Skip("migrates down to an empty schema: only runs against a disposable testcontainers database")
	}
	ctx := context.Background()
	pool, dsn, cleanup, err := testutil.StartPostgresWithDSN(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	// back to an empty schema
	mg, err := database.NewMigrator(migrations.FS, dsn)
	require.NoError(t, err)
	current, _, err := mg.Version()
	require.NoError(t, err)
	require.NoError(t, mg.Down(int(current)))
	require.NoError(t, mg.Close())

	const instances = 5
	versions := make([]uint, instances)
	errs := make([]error, instances)
	var wg sync.WaitGroup
	for i := range instances {
		wg.Add(1)
		go func() {
			defer wg.Done()
			versions[i], errs[i] = database.MigrateUp(migrations.FS, dsn)
		}()
	}
	wg.Wait()

	for i := range instances {
		require.NoError(t, errs[i], "instance %d", i)
		assert.Equal(t, uint(2), versions[i], "instance %d", i)
	}

	// running again is a no-op
	v, err := database.MigrateUp(migrations.FS, dsn)
	require.NoError(t, err)
	assert.Equal(t, uint(2), v)

	var tables int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name IN ('products','audit_logs')`).Scan(&tables))
	assert.Equal(t, 2, tables)
}
