package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog writes one structured log line per HTTP request.
// 5xx is logged at ERROR, 4xx at WARN, everything else at INFO.
// Paths starting with any of skipPrefixes (e.g. /health, /api-docs) are not logged.
func AccessLog(logger *slog.Logger, skipPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, p := range skipPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = path // unmatched route (404)
		}

		level := slog.LevelInfo
		switch {
		case status >= http.StatusInternalServerError:
			level = slog.LevelError
		case status >= http.StatusBadRequest:
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("response_bytes", c.Writer.Size()),
		}
		if code, ok := c.Get(ContextKeyErrorCode); ok {
			attrs = append(attrs, slog.Any("error_code", code))
		}
		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// ContextKeyErrorCode is set by handlers so the access log can include the business error code.
const ContextKeyErrorCode = "error_code"
