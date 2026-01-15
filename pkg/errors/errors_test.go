package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *AppError
		wantMsg string
	}{
		{
			name: "error with underlying error",
			err: &AppError{
				Type:    ErrorTypeConfig,
				Message: "invalid config",
				Err:     errors.New("file not found"),
				Op:      "loadConfig",
			},
			wantMsg: "loadConfig: invalid config: file not found",
		},
		{
			name: "error without underlying error",
			err: &AppError{
				Type:    ErrorTypeValidation,
				Message: "invalid input",
				Op:      "validate",
			},
			wantMsg: "validate: invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &AppError{
		Type:    ErrorTypeDB,
		Message: "db error",
		Err:     underlying,
		Op:      "connect",
	}

	got := err.Unwrap()
	if got != underlying {
		t.Errorf("Unwrap() = %v, want %v", got, underlying)
	}
}

func TestAppError_UnwrapNil(t *testing.T) {
	err := &AppError{
		Type:    ErrorTypeDB,
		Message: "db error",
		Op:      "connect",
	}

	got := err.Unwrap()
	if got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestNewConfigError(t *testing.T) {
	underlying := errors.New("file error")
	err := NewConfigError("loadConfig", "failed to load", underlying)

	if err.Type != ErrorTypeConfig {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeConfig)
	}
	if err.Message != "failed to load" {
		t.Errorf("Message = %q, want %q", err.Message, "failed to load")
	}
	if err.Op != "loadConfig" {
		t.Errorf("Op = %q, want %q", err.Op, "loadConfig")
	}
	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestNewDBError(t *testing.T) {
	underlying := errors.New("connection failed")
	err := NewDBError("connect", "connection failed", underlying)

	if err.Type != ErrorTypeDB {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeDB)
	}
	if err.Message != "connection failed" {
		t.Errorf("Message = %q, want %q", err.Message, "connection failed")
	}
	if err.Op != "connect" {
		t.Errorf("Op = %q, want %q", err.Op, "connect")
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("validate", "invalid input", nil)

	if err.Type != ErrorTypeValidation {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
	}
	if err.Message != "invalid input" {
		t.Errorf("Message = %q, want %q", err.Message, "invalid input")
	}
}

func TestNewExportError(t *testing.T) {
	underlying := errors.New("write failed")
	err := NewExportError("export", "export failed", underlying)

	if err.Type != ErrorTypeExport {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeExport)
	}
	if err.Message != "export failed" {
		t.Errorf("Message = %q, want %q", err.Message, "export failed")
	}
}

func TestNewIOError(t *testing.T) {
	underlying := errors.New("permission denied")
	err := NewIOError("writeFile", "write failed", underlying)

	if err.Type != ErrorTypeIO {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeIO)
	}
	if err.Message != "write failed" {
		t.Errorf("Message = %q, want %q", err.Message, "write failed")
	}
}

func TestNewStateError(t *testing.T) {
	underlying := errors.New("corrupted state")
	err := NewStateError("loadState", "state file invalid", underlying)

	if err.Type != ErrorTypeState {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeState)
	}
	if err.Message != "state file invalid" {
		t.Errorf("Message = %q, want %q", err.Message, "state file invalid")
	}
}

func TestIsType(t *testing.T) {
	configErr := NewConfigError("test", "config error", nil)
	dbErr := NewDBError("test", "db error", nil)
	standardErr := errors.New("standard error")

	tests := []struct {
		name      string
		err       error
		errorType ErrorType
		want      bool
	}{
		{
			name:      "config error is config type",
			err:       configErr,
			errorType: ErrorTypeConfig,
			want:      true,
		},
		{
			name:      "db error is not config type",
			err:       dbErr,
			errorType: ErrorTypeConfig,
			want:      false,
		},
		{
			name:      "db error is db type",
			err:       dbErr,
			errorType: ErrorTypeDB,
			want:      true,
		},
		{
			name:      "standard error is not config type",
			err:       standardErr,
			errorType: ErrorTypeConfig,
			want:      false,
		},
		{
			name:      "nil error",
			err:       nil,
			errorType: ErrorTypeConfig,
			want:      false,
		},
		{
			name:      "wrapped config error",
			err:       fmt.Errorf("wrapped: %w", configErr),
			errorType: ErrorTypeConfig,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsType(tt.err, tt.errorType)
			if got != tt.want {
				t.Errorf("IsType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOp(t *testing.T) {
	err := NewConfigError("loadConfig", "failed", nil)

	got := GetOp(err)
	if got != "loadConfig" {
		t.Errorf("GetOp() = %q, want %q", got, "loadConfig")
	}
}

func TestGetOp_NilError(t *testing.T) {
	got := GetOp(nil)
	if got != "" {
		t.Errorf("GetOp() = %q, want empty string", got)
	}
}

func TestGetOp_StandardError(t *testing.T) {
	got := GetOp(errors.New("standard error"))
	if got != "" {
		t.Errorf("GetOp() = %q, want empty string", got)
	}
}

func TestGetOp_WrappedError(t *testing.T) {
	err := NewConfigError("loadConfig", "failed", nil)
	wrapped := fmt.Errorf("wrapped: %w", err)

	got := GetOp(wrapped)
	if got != "loadConfig" {
		t.Errorf("GetOp() = %q, want %q", got, "loadConfig")
	}
}
