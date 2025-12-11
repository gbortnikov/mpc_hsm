package logger

import (
	"context"
	"log/slog"
)

// ContextKey тип для ключей контекста
type contextKey string

const (
	// RequestIDKey ключ контекста для ID запросов
	RequestIDKey contextKey = "request_id"
	// SessionIDKey ключ контекста для ID сессий
	SessionIDKey contextKey = "session_id"
	// PartyIDKey ключ контекста для ID участников
	PartyIDKey contextKey = "party_id"
)

// Logger обёртка над slog.Logger с поддержкой контекста
type Logger struct {
	logger *slog.Logger
}

// New создаёт новый контекстный логгер
func New(logger *slog.Logger) *Logger {
	return &Logger{logger: logger}
}

// WithContext извлекает контекстную информацию и возвращает логгер с этими атрибутами
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
