// Package middleware contains gin middlewares for request context, logging and panic recovery.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/product-service/pkg/requestctx"
)

const (
	HeaderRequestID = "X-Request-ID"
	HeaderUserID    = "X-User-ID"

	AnonymousActor = "anonymous"
)

// safeHeaderValue limits client-supplied IDs to a safe charset and length (prevents log injection).
var safeHeaderValue = regexp.MustCompile(`^[A-Za-z0-9._:@-]{1,128}$`)

// RequestID propagates X-Request-ID (or generates one), stores it in the request
// context and echoes it in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !safeHeaderValue.MatchString(id) {
			id = newRequestID()
		}
		c.Request = c.Request.WithContext(requestctx.WithRequestID(c.Request.Context(), id))
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// Actor identifies who performs the request from X-User-ID.
// There is no authentication in this service yet; plug the authenticated user in here later.
func Actor() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := c.GetHeader(HeaderUserID)
		if !safeHeaderValue.MatchString(actor) {
			actor = AnonymousActor
		}
		c.Request = c.Request.WithContext(requestctx.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
