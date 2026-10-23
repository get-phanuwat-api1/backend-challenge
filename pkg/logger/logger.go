// Package logger builds the application's structured logger (log/slog).
// Every record logged with a context automatically gets request_id and user_id attributes.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

// Options configures the logger.
type Options struct {
	Level   string // debug | info | warn | error (default info)
	Format  string // json | text (default json)
	Service string // added as "service" attribute on every record
}

// NewLogger creates a slog.Logger writing to w.
func NewLogger(w io.Writer, opts Options) (*slog.Logger, error) {
	level, err := ParseLevel(opts.Level)
	if err != nil {
		return nil, err
	}
	handlerOpts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	switch strings.ToLower(opts.Format) {
	case "", "json":
		h = slog.NewJSONHandler(w, handlerOpts)
	case "text":
		h = slog.NewTextHandler(w, handlerOpts)
	default:
		return nil, fmt.Errorf("unknown log format %q (want json or text)", opts.Format)
	}

	l := slog.New(&contextHandler{Handler: h})
	if opts.Service != "" {
		l = l.With(slog.String("service", opts.Service))
	}
	return l, nil
}

// NewDiscardLogger returns a logger that drops everything (useful in tests).
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ParseLevel converts a level name to slog.Level. Empty means info.
func ParseLevel(s string) (slog.Level, error) {
	if s == "" {
		return slog.LevelInfo, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("invalid log level %q: %w", s, err)
	}
	return level, nil
}

// contextHandler enriches records with request-scoped values from the context.
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := requestctx.RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if userID := requestctx.UserID(ctx); userID != "" {
		r.AddAttrs(slog.String("user_id", userID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
