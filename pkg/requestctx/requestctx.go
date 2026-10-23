// Package requestctx stores per-request metadata (request ID, authenticated user) in context.Context.
package requestctx

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	userIDKey
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

// WithUserID returns a copy of ctx carrying the authenticated user ID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID returns the authenticated user ID stored in ctx, or "".
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
