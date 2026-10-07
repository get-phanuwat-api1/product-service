package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/middleware"
	"github.com/phanuwat/product-service/pkg/logger"
	"github.com/phanuwat/product-service/pkg/requestctx"
)

func newEngine(t *testing.T, buf *bytes.Buffer) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	l, err := logger.NewLogger(buf, logger.Options{Level: "debug"})
	require.NoError(t, err)

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Actor(), middleware.AccessLog(l, "/health"), middleware.Recovery(l))
	r.GET("/echo", func(c *gin.Context) {
		ctx := c.Request.Context()
		c.JSON(http.StatusOK, gin.H{"request_id": requestctx.RequestID(ctx), "actor": requestctx.Actor(ctx)})
	})
	r.GET("/bad", func(c *gin.Context) {
		c.Set(middleware.ContextKeyErrorCode, "INVALID_NAME")
		c.Status(http.StatusBadRequest)
	})
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func get(r http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		out = append(out, m)
	}
	return out
}

func TestRequestID_PropagatesIncomingHeader(t *testing.T) {
	var buf bytes.Buffer
	w := get(newEngine(t, &buf), "/echo", map[string]string{"X-Request-ID": "abc-123", "X-User-ID": "user-1"})

	assert.Equal(t, "abc-123", w.Header().Get("X-Request-ID"))
	assert.JSONEq(t, `{"request_id":"abc-123","actor":"user-1"}`, w.Body.String())
}

func TestRequestID_GeneratesWhenMissingOrUnsafe(t *testing.T) {
	var buf bytes.Buffer
	r := newEngine(t, &buf)

	for _, incoming := range []string{"", "bad value\nwith newline"} {
		w := get(r, "/echo", map[string]string{"X-Request-ID": incoming, "X-User-ID": "evil\nuser"})
		id := w.Header().Get("X-Request-ID")
		assert.Len(t, id, 32)
		assert.NotEqual(t, incoming, id)
		assert.Contains(t, w.Body.String(), `"actor":"anonymous"`)
	}
}

func TestAccessLog_WritesRequestLine(t *testing.T) {
	var buf bytes.Buffer
	get(newEngine(t, &buf), "/echo", map[string]string{"X-Request-ID": "req-9", "X-User-ID": "user-9"})

	lines := logLines(t, &buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "http request", lines[0]["msg"])
	assert.Equal(t, "INFO", lines[0]["level"])
	assert.Equal(t, "GET", lines[0]["method"])
	assert.Equal(t, "/echo", lines[0]["route"])
	assert.Equal(t, float64(200), lines[0]["status"])
	assert.Equal(t, "req-9", lines[0]["request_id"])
	assert.Equal(t, "user-9", lines[0]["actor"])
	assert.Contains(t, lines[0], "latency_ms")
}

func TestAccessLog_WarnOn4xxWithErrorCode(t *testing.T) {
	var buf bytes.Buffer
	get(newEngine(t, &buf), "/bad", nil)

	lines := logLines(t, &buf)
	require.Len(t, lines, 1)
	assert.Equal(t, "WARN", lines[0]["level"])
	assert.Equal(t, "INVALID_NAME", lines[0]["error_code"])
}

func TestAccessLog_SkipsPrefixes(t *testing.T) {
	var buf bytes.Buffer
	get(newEngine(t, &buf), "/health", nil)
	assert.Empty(t, buf.String())
}

func TestRecovery_Returns500AndLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	w := get(newEngine(t, &buf), "/panic", map[string]string{"X-Request-ID": "req-p"})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"successful":false,"error_code":"INTERNAL_SERVER_ERROR"}`, w.Body.String())

	lines := logLines(t, &buf)
	require.Len(t, lines, 2) // panic line + access log line
	assert.Equal(t, "panic recovered", lines[0]["msg"])
	assert.Equal(t, "boom", lines[0]["panic"])
	assert.Equal(t, "req-p", lines[0]["request_id"])
	assert.Equal(t, "ERROR", lines[1]["level"])
	assert.Equal(t, float64(500), lines[1]["status"])
}
