package errors

import (
	"errors"
	"fmt"
)

// Error codes for application errors
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

// AppError represents a structured application error
type AppError struct {
	Code    string // Error code for programmatic handling
	Message string // Human-readable error message
	Err     error  // Underlying error
	Details map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetail adds a detail to the error
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New creates a new AppError
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Is checks if an error matches a specific code
func Is(err error, code string) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

// Common error constructors

// Internal creates an internal error
func Internal(message string, err error) *AppError {
	return Wrap(err, CodeInternal, message)
}

// InvalidInput creates an invalid input error
func InvalidInput(message string) *AppError {
	return New(CodeInvalidInput, message)
}

// NotFound creates a not found error
func NotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

// AlreadyExists creates an already exists error
func AlreadyExists(resource string) *AppError {
	return New(CodeAlreadyExists, fmt.Sprintf("%s already exists", resource))
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message)
}

// Timeout creates a timeout error
func Timeout(operation string) *AppError {
	return New(CodeTimeout, fmt.Sprintf("%s timed out", operation))
}

// SessionError creates a session error
func SessionError(message string, err error) *AppError {
	return Wrap(err, CodeSessionError, message)
}

// CryptoError creates a crypto error
func CryptoError(message string, err error) *AppError {
	return Wrap(err, CodeCryptoError, message)
}

// DatabaseError creates a database error
func DatabaseError(message string, err error) *AppError {
	return Wrap(err, CodeDatabaseError, message)
}

// NetworkError creates a network error
func NetworkError(message string, err error) *AppError {
	return Wrap(err, CodeNetworkError, message)
}

// VerificationError creates a verification error
func VerificationError(message string) *AppError {
	return New(CodeVerification, message)
}
