package database_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/migrations"
	"github.com/get-phanuwat-api1/product-service/pkg/database"
)

func TestLoadMigrations_ProjectFiles(t *testing.T) {
	t.Parallel()
	migs, err := database.LoadMigrations(migrations.FS)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(migs), 2)

	assert.Equal(t, int64(1), migs[0].Version)
	assert.Equal(t, "create products table", migs[0].Description)
	assert.Equal(t, int64(2), migs[1].Version)
	assert.Equal(t, "create audit logs table", migs[1].Description)
	for i, m := range migs {
		assert.Len(t, m.Checksum, 64)
		assert.NotEmpty(t, m.UpSQL)
		assert.NotEmpty(t, m.DownSQL, "every project migration should be reversible")
		if i > 0 {
			assert.Greater(t, m.Version, migs[i-1].Version, "sorted by version")
		}
	}
}

func TestLoadMigrations_Parsing(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"000010_add_sku.up.sql":    {Data: []byte("ALTER TABLE t ADD sku text;")},
		"000002_create_t.up.sql":   {Data: []byte("CREATE TABLE t (id int);")},
		"000002_create_t.down.sql": {Data: []byte("DROP TABLE t;")},
		"embed.go":                 {Data: []byte("package x")},
		"README.md":                {Data: []byte("ignored")},
	}
	migs, err := database.LoadMigrations(fsys)
	require.NoError(t, err)
	require.Len(t, migs, 2)

	assert.Equal(t, int64(2), migs[0].Version)
	assert.Equal(t, "create t", migs[0].Description)
	assert.Equal(t, "DROP TABLE t;", migs[0].DownSQL)
	assert.Equal(t, int64(10), migs[1].Version)
	assert.Equal(t, "add sku", migs[1].Description)
	assert.Empty(t, migs[1].DownSQL)
}

func TestLoadMigrations_ChecksumChangesWithContent(t *testing.T) {
	t.Parallel()
	a, err := database.LoadMigrations(fstest.MapFS{"000001_x.up.sql": {Data: []byte("SELECT 1;")}})
	require.NoError(t, err)
	b, err := database.LoadMigrations(fstest.MapFS{"000001_x.up.sql": {Data: []byte("SELECT 2;")}})
	require.NoError(t, err)
	assert.NotEqual(t, a[0].Checksum, b[0].Checksum)
}

func TestLoadMigrations_Invalid(t *testing.T) {
	t.Parallel()
	tests := map[string]fstest.MapFS{
		"bad file name":   {"create_products.sql": {Data: []byte("x")}},
		"upper case name": {"000001_Create.up.sql": {Data: []byte("x")}},
		"version zero":    {"000000_x.up.sql": {Data: []byte("x")}},
		"down without up": {"000001_x.down.sql": {Data: []byte("x")}},
		"empty up":        {"000001_x.up.sql": {Data: []byte("  \n")}},
		"two names same version": {
			"000001_a.up.sql": {Data: []byte("x")},
			"000001_b.up.sql": {Data: []byte("y")},
		},
	}
	for name, fsys := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := database.LoadMigrations(fsys)
			assert.Error(t, err)
		})
	}
}
