package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/config"
)

func TestLoadFile_DefaultYAML(t *testing.T) {
	cfg, err := config.LoadFile("config.yaml")
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t,
		"postgres://postgres:postgres@localhost:5432/product?pool_max_conns=10&sslmode=disable",
		cfg.Database.DSN())
}

func TestLoadFile_EnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_HOST", "db.internal")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DATABASE_PASSWORD", "p@ss/word")

	cfg, err := config.LoadFile("config.yaml")
	require.NoError(t, err)

	assert.Equal(t, "db.internal", cfg.Database.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Contains(t, cfg.Database.DSN(), "postgres:p%40ss%2Fword@db.internal:5432")
}

func TestLoadFile_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: 0\ndatabase:\n  host: ''\n"), 0o600))

	_, err := config.LoadFile(path)
	assert.ErrorContains(t, err, "server.port")
	assert.ErrorContains(t, err, "database.host")
}

func TestLoadFile_Missing(t *testing.T) {
	_, err := config.LoadFile("does-not-exist.yaml")
	assert.Error(t, err)
}
