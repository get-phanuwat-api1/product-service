package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/pkg/logger"
	"github.com/get-phanuwat-api1/product-service/pkg/requestctx"
)

func TestNewLogger_AddsContextAttributes(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{Level: "info", Format: "json", Service: "product-service"})
	require.NoError(t, err)

	ctx := requestctx.WithActor(requestctx.WithRequestID(context.Background(), "req-1"), "user-9")
	l.With("component", "test").InfoContext(ctx, "hello", "key", "value")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.Equal(t, "hello", rec["msg"])
	assert.Equal(t, "INFO", rec["level"])
	assert.Equal(t, "product-service", rec["service"])
	assert.Equal(t, "test", rec["component"])
	assert.Equal(t, "value", rec["key"])
	assert.Equal(t, "req-1", rec["request_id"])
	assert.Equal(t, "user-9", rec["actor"])
}

func TestNewLogger_NoContextValues(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{})
	require.NoError(t, err)

	l.Info("plain")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.NotContains(t, rec, "request_id")
	assert.NotContains(t, rec, "actor")
}

func TestNewLogger_LevelFilter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{Level: "warn"})
	require.NoError(t, err)

	l.Info("dropped")
	assert.Empty(t, buf.String())
	l.Warn("kept")
	assert.Contains(t, buf.String(), "kept")
}

func TestNewLogger_TextFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{Format: "text"})
	require.NoError(t, err)
	l.Info("hello")
	assert.Contains(t, buf.String(), "msg=hello")
}

func TestNewLogger_InvalidOptions(t *testing.T) {
	t.Parallel()
	_, err := logger.NewLogger(&bytes.Buffer{}, logger.Options{Level: "loud"})
	assert.Error(t, err)
	_, err = logger.NewLogger(&bytes.Buffer{}, logger.Options{Format: "xml"})
	assert.Error(t, err)
}

func TestParseLevel(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]slog.Level{
		"": slog.LevelInfo, "debug": slog.LevelDebug, "INFO": slog.LevelInfo,
		"warn": slog.LevelWarn, "error": slog.LevelError,
	} {
		got, err := logger.ParseLevel(in)
		require.NoError(t, err)
		assert.Equal(t, want, got, in)
	}
}
