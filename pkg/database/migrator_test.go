package database_test

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/migrations"
	"github.com/get-phanuwat-api1/product-service/pkg/database"
	"github.com/get-phanuwat-api1/product-service/test/testutil"
)

var testDSN string

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() || os.Getenv("TEST_DATABASE_URL") != "" {
		// DB tests below create and drop schemas: only run them on a disposable container.
		os.Exit(m.Run())
	}
	pool, dsn, cleanup, err := testutil.StartPostgresWithDSN(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot start postgres (is Docker running?): %v\n", err)
		os.Exit(1)
	}
	pool.Close()
	testDSN = dsn
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// isolatedPool returns a pool whose search_path is a fresh, empty schema,
// so every test starts with no tables and no schema_migrations.
func isolatedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testDSN == "" {
		t.Skip("needs Docker (disposable testcontainers database)")
	}
	ctx := context.Background()
	schema := "t_" + strings.ToLower(strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(t.Name()))

	admin, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE; CREATE SCHEMA %q`, schema, schema))
	require.NoError(t, err)

	cfg, err := pgxpool.ParseConfig(testDSN)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
		admin.Close()
	})
	return pool
}

// withFiles returns the project migrations plus/minus extra files.
func withFiles(t *testing.T, extra map[string]string, remove ...string) fs.FS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)
	for _, e := range entries {
		b, err := fs.ReadFile(migrations.FS, e.Name())
		require.NoError(t, err)
		out[e.Name()] = &fstest.MapFile{Data: b}
	}
	for name, body := range extra {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	for _, name := range remove {
		delete(out, name)
	}
	return out
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok))
	return ok
}

func versions(h []database.AppliedMigration) []int64 {
	out := make([]int64, len(h))
	for i, m := range h {
		out[i] = m.Version
	}
	return out
}

func TestMigrator_UpRecordsOneRowPerMigration(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	mg, err := database.NewMigrator(pool, migrations.FS)
	require.NoError(t, err)

	applied, err := mg.Up(ctx)
	require.NoError(t, err)
	require.Len(t, applied, 2)

	history, err := mg.History(ctx)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, int64(1), history[0].Version)
	assert.Equal(t, "create products table", history[0].Description)
	assert.Equal(t, int64(2), history[1].Version)
	assert.Equal(t, "create audit logs table", history[1].Description)
	for _, h := range history {
		assert.Len(t, h.Checksum, 64)
		assert.False(t, h.AppliedAt.IsZero())
	}
	assert.True(t, tableExists(t, pool, "products"))
	assert.True(t, tableExists(t, pool, "audit_logs"))

	// second run is a no-op
	applied, err = mg.Up(ctx)
	require.NoError(t, err)
	assert.Empty(t, applied)
}

func TestMigrator_AppliesOnlyNewMigrations(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	_, err := database.MigrateUp(ctx, pool, migrations.FS)
	require.NoError(t, err)

	next := withFiles(t, map[string]string{
		"000003_add_sku_to_products.up.sql":   "ALTER TABLE products ADD COLUMN sku TEXT;",
		"000003_add_sku_to_products.down.sql": "ALTER TABLE products DROP COLUMN sku;",
	})
	applied, err := database.MigrateUp(ctx, pool, next)
	require.NoError(t, err)
	require.Len(t, applied, 1)
	assert.Equal(t, int64(3), applied[0].Version)
	assert.Equal(t, "add sku to products", applied[0].Description)
}

func TestMigrator_DownRevertsLatest(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	mg, err := database.NewMigrator(pool, migrations.FS)
	require.NoError(t, err)
	_, err = mg.Up(ctx)
	require.NoError(t, err)

	reverted, err := mg.Down(ctx, 1)
	require.NoError(t, err)
	require.Len(t, reverted, 1)
	assert.Equal(t, int64(2), reverted[0].Version)
	assert.False(t, tableExists(t, pool, "audit_logs"))
	assert.True(t, tableExists(t, pool, "products"))

	history, err := mg.History(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, versions(history))

	pending, err := mg.Pending(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(2), pending[0].Version)

	_, err = mg.Up(ctx)
	require.NoError(t, err)
	assert.True(t, tableExists(t, pool, "audit_logs"))
}

func TestMigrator_FailedMigrationIsRolledBack(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()

	broken := withFiles(t, map[string]string{
		"000003_broken.up.sql": "CREATE TABLE half_done (id INT); SELECT this_function_does_not_exist();",
	})
	applied, err := database.MigrateUp(ctx, pool, broken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apply migration 3 (broken)")
	assert.Len(t, applied, 2, "earlier migrations stay applied")

	assert.False(t, tableExists(t, pool, "half_done"), "partial DDL must be rolled back")
	mg, err := database.NewMigrator(pool, broken)
	require.NoError(t, err)
	history, err := mg.History(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, versions(history))
}

func TestMigrator_RefusesEditedOrMissingAppliedMigration(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	_, err := database.MigrateUp(ctx, pool, migrations.FS)
	require.NoError(t, err)

	edited := withFiles(t, map[string]string{
		"000001_create_products_table.up.sql": "CREATE TABLE products (id INT); -- edited",
	})
	_, err = database.MigrateUp(ctx, pool, edited)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "was modified after it was applied")

	missing := withFiles(t, nil, "000002_create_audit_logs_table.up.sql", "000002_create_audit_logs_table.down.sql")
	_, err = database.MigrateUp(ctx, pool, missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file is missing")
}

func TestMigrator_ConvertsLegacyGolangMigrateTable(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	migs, err := database.LoadMigrations(migrations.FS)
	require.NoError(t, err)

	// a database migrated to version 1 by golang-migrate
	_, err = pool.Exec(ctx, migs[0].UpSQL)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL);
		INSERT INTO schema_migrations VALUES (1, false)`)
	require.NoError(t, err)

	applied, err := database.MigrateUp(ctx, pool, migrations.FS)
	require.NoError(t, err)
	require.Len(t, applied, 1, "version 1 is kept, only version 2 runs")
	assert.Equal(t, int64(2), applied[0].Version)

	mg, err := database.NewMigrator(pool, migrations.FS)
	require.NoError(t, err)
	history, err := mg.History(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, versions(history))
	assert.Equal(t, "create products table", history[0].Description)
}

func TestMigrator_RefusesDirtyLegacyTable(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL);
		INSERT INTO schema_migrations VALUES (2, true)`)
	require.NoError(t, err)

	_, err = database.MigrateUp(ctx, pool, migrations.FS)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dirty")
}

// Several API instances starting at once with auto-migrate: the advisory lock makes
// exactly one of them apply each migration and the others see nothing pending.
func TestMigrator_ConcurrentUpIsSafe(t *testing.T) {
	pool := isolatedPool(t)
	ctx := context.Background()

	const instances = 5
	results := make([][]database.Migration, instances)
	errs := make([]error, instances)
	var wg sync.WaitGroup
	for i := range instances {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = database.MigrateUp(ctx, pool, migrations.FS)
		}()
	}
	wg.Wait()

	total := 0
	for i := range instances {
		require.NoError(t, errs[i], "instance %d", i)
		total += len(results[i])
	}
	assert.Equal(t, 2, total, "each migration applied exactly once")
}
