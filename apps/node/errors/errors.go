package errors

import (
	"errors"
	"fmt"
)

// Коды ошибок приложения
const (
	CodeInternal       = "INTERNAL_ERROR"
	CodeInvalidInput   = "INVALID_INPUT"
	CodeNotFound       = "NOT_FOUND"
	CodeAlreadyExists  = "ALREADY_EXISTS"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeTimeout        = "TIMEOUT"
	CodeSessionError   = "SESSION_ERROR"
	CodeCryptoError    = "CRYPTO_ERROR"
	CodeDatabaseError  = "DATABASE_ERROR"
	CodeNetworkError   = "NETWORK_ERROR"
	CodeVerification   = "VERIFICATION_ERROR"
)

// AppError представляет структурированную ошибку приложения
type AppError struct {
	Code    string                 // Код ошибки для программной обработки
	Message string                 // Читаемое сообщение об ошибке
	Err     error                  // Базовая ошибка
	Details map[string]interface{} // Дополнительный контекст
}

// Error реализует интерфейс error
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap возвращает базовую ошибку
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetail добавляет детали к ошибке
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New создаёт новую AppError
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap оборачивает существующую ошибку с дополнительным контекстом
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Is проверяет, соответствует ли ошибка определённому коду
func Is(err error, code string) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// GetCode извлекает код ошибки из ошибки
func GetCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

// Конструкторы типичных ошибок

// Internal создаёт внутреннюю ошибку
func Internal(message string, err error) *AppError {
	return Wrap(err, CodeInternal, message)
}

// InvalidInput создаёт ошибку некорректного ввода
func InvalidInput(message string) *AppError {
	return New(CodeInvalidInput, message)
}

// NotFound создаёт ошибку "не найдено"
func NotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

// AlreadyExists создаёт ошибку "уже существует"
func AlreadyExists(resource string) *AppError {
	return New(CodeAlreadyExists, fmt.Sprintf("%s already exists", resource))
}

// Unauthorized создаёт ошибку авторизации
func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message)
}

// Timeout создаёт ошибку таймаута
func Timeout(operation string) *AppError {
	return New(CodeTimeout, fmt.Sprintf("%s timed out", operation))
}

// SessionError создаёт ошибку сессии
func SessionError(message string, err error) *AppError {
	return Wrap(err, CodeSessionError, message)
}

// CryptoError создаёт криптографическую ошибку
func CryptoError(message string, err error) *AppError {
	return Wrap(err, CodeCryptoError, message)
}

// DatabaseError создаёт ошибку базы данных
func DatabaseError(message string, err error) *AppError {
	return Wrap(err, CodeDatabaseError, message)
}

// NetworkError создаёт сетевую ошибку
func NetworkError(message string, err error) *AppError {
	return Wrap(err, CodeNetworkError, message)
}

// VerificationError создаёт ошибку верификации
func VerificationError(message string) *AppError {
	return New(CodeVerification, message)
}
