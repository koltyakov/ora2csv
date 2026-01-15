package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeConfig     ErrorType = "config"
	ErrorTypeDB         ErrorType = "database"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeExport     ErrorType = "export"
	ErrorTypeIO         ErrorType = "io"
	ErrorTypeState      ErrorType = "state"
)

// AppError is a structured error with context
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
	Op      string // Operation that failed
}

// Error returns the error message
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new configuration error
func NewConfigError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeConfig,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// NewDBError creates a new database error
func NewDBError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeDB,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// NewExportError creates a new export error
func NewExportError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeExport,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// NewIOError creates a new I/O error
func NewIOError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeIO,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// NewStateError creates a new state error
func NewStateError(op, message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeState,
		Message: message,
		Err:     err,
		Op:      op,
	}
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	var appErr *AppError
	if err == nil {
		return false
	}
	ok := errors.As(err, &appErr)
	return ok && appErr.Type == errorType
}

// GetOp returns the operation from an error, if available
func GetOp(err error) string {
	var appErr *AppError
	if err == nil {
		return ""
	}
	if errors.As(err, &appErr) {
		return appErr.Op
	}
	return ""
}
