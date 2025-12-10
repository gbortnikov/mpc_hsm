package logger

import (
	"context"
	"log/slog"
)

// ContextKey type for context keys
type contextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey contextKey = "request_id"
	// SessionIDKey is the context key for session IDs
	SessionIDKey contextKey = "session_id"
	// PartyIDKey is the context key for party IDs
	PartyIDKey contextKey = "party_id"
)

// Logger wraps slog.Logger with context awareness
type Logger struct {
	logger *slog.Logger
}

// New creates a new contextual logger
func New(logger *slog.Logger) *Logger {
	return &Logger{logger: logger}
}

// WithContext extracts contextual information and returns a logger with those attributes
func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
	attrs := make([]any, 0, 6)

	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		attrs = append(attrs, "request_id", requestID)
	}
	if sessionID := ctx.Value(SessionIDKey); sessionID != nil {
		attrs = append(attrs, "session_id", sessionID)
	}
	if partyID := ctx.Value(PartyIDKey); partyID != nil {
		attrs = append(attrs, "party_id", partyID)
	}

	if len(attrs) > 0 {
		return l.logger.With(attrs...)
	}
	return l.logger
}

// Debug logs a debug message with context
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Debug(msg, args...)
}

// Info logs an info message with context
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Info(msg, args...)
}

// Warn logs a warning message with context
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Warn(msg, args...)
}

// Error logs an error message with context
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Error(msg, args...)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithSessionID adds a session ID to the context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// WithPartyID adds a party ID to the context
func WithPartyID(ctx context.Context, partyID string) context.Context {
	return context.WithValue(ctx, PartyIDKey, partyID)
}
