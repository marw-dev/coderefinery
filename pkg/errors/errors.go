package errors

import (
	"fmt"
	"net/http"
	"runtime"
)

type ErrorCode string

const (
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeConflict       ErrorCode = "CONFLICT"
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavail ErrorCode = "SERVICE_UNAVAILABLE"
)

// AppError ist unser Standard-Error für die gesamte App
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]any 		  `json:"details,omitempty"`
	Internal   error                  `json:"-"` // Nicht als JSON ausgeben
	StackTrace string                 `json:"-"`
}

// Error implementiert das error Interface
func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("[%s] %s (cause: %v)", e.Code, e.Message, e.Internal)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Internal
}

// HTTPStatus mappt ErrorCodes zu HTTP Status Codes
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeServiceUnavail:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// New erstellt einen neuen AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Details:    make(map[string]any),
		StackTrace: captureStackTrace(),
	}
}

// Wrap verpackt einen existierenden Error (z.B. von Datenbank)
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Internal:   err,
		Details:    make(map[string]any),
		StackTrace: captureStackTrace(),
	}
}

// Hilfsfunktion für Stacktrace
func captureStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
