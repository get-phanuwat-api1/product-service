package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/phanuwat/product-service/internal/apperror"
)

// Recovery turns panics into a logged error and a 500 response in the API's standard shape.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					slog.String("panic", fmt.Sprint(r)),
					slog.String("stack", string(debug.Stack())),
				)
				c.Set(ContextKeyErrorCode, apperror.CodeInternal)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"successful": false,
					"error_code": apperror.CodeInternal,
				})
			}
		}()
		c.Next()
	}
}
