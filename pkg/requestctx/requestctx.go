// Package requestctx stores per-request metadata (request ID, actor) in context.Context.
package requestctx

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	actorKey
)

// WithRequestID returns a copy of ctx carrying the request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestID returns the request ID stored in ctx, or "".
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// WithActor returns a copy of ctx carrying the actor (who performs the request).
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// Actor returns the actor stored in ctx, or "".
func Actor(ctx context.Context) string {
	v, _ := ctx.Value(actorKey).(string)
	return v
}
