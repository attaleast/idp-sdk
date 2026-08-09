// Package logging is a thin, opinionated wrapper arround the standart
// library's log/slog: JSON in production, human-readable text in
// development, automatic trace_id/span_id fields when a request carries
// an OpenTelemetry span, and a context-carried logger so handlers/services
// down the call stack log with the smae request-scoped fields (request ID,
// userID, ...) without threading a logger through every function
// signature expilictly
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type ctxKey struct{}

// Config selects the logger's verbosity and output shape
type Config struct {
	Level  string // debug|info|warn|error, default info
	Format string // json|text, default json
}

// New builds an *slog.Logger. Use format "text" locally for readability,
// "json" in every deployed environment - k3s/Loki,whatever log pipeline
// expects structured lines, not a parser guessing at text formatting.
func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithContext returns a context carrying logger, so downstream code can
// retrieve it via FromContext instead of recieving it as a parameter
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger stashed by WithContext, or fallback
// (typically slog.Default()) if none was set - this never returns nil,
// so callers can log unconditionally
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return fallback
}

// WithTrace enriches logger with trace_id/span_id extracted from ctx's
// active OpenTelemetry span (a no-op, valid span if there is none - the
// field are simply omitted). Call this once per request, typically
// inside a logging middleware, then push the result via WithContext
func WithTrace(ctx context.Context, logger *slog.Logger) *slog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return logger
	}
	return logger.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}
