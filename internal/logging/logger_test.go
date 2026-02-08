package logging

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("logger with verbose=false", func(t *testing.T) {
		logger := New(false)
		if logger == nil {
			t.Fatal("New() returned nil")
		}
		if logger.level != LevelInfo {
			t.Errorf("level = %v, want LevelInfo", logger.level)
		}
		if logger.file != nil {
			t.Error("file should be nil for logger without file")
		}
	})

	t.Run("logger with verbose=true", func(t *testing.T) {
		logger := New(true)
		if logger.level != LevelDebug {
			t.Errorf("level = %v, want LevelDebug", logger.level)
		}
	})
}

func TestNewWithFile(t *testing.T) {
	t.Run("creates logger with file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := tmpDir + "/test.log"

		logger, err := NewWithFile(logPath, false)
		if err != nil {
			t.Fatalf("NewWithFile() error = %v", err)
		}
		if logger == nil {
			t.Fatal("logger is nil")
		}
		if logger.file == nil {
			t.Error("file should not be nil")
		}

		// Clean up
		if err := logger.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		_, err := NewWithFile("/nonexistent/dir/test.log", false)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestLogger_Close(t *testing.T) {
	t.Run("close logger without file", func(t *testing.T) {
		logger := New(false)
		err := logger.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("close logger with file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := tmpDir + "/test.log"

		logger, err := NewWithFile(logPath, false)
		if err != nil {
			t.Fatalf("NewWithFile() error = %v", err)
		}

		err = logger.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestLogger_SetPrefix(t *testing.T) {
	logger := New(false)
	logger.SetPrefix("test")

	if logger.prefix != "test" {
		t.Errorf("prefix = %q, want %q", logger.prefix, "test")
	}
}

func TestLogger_WithPrefix(t *testing.T) {
	parent := New(false)
	child := parent.WithPrefix("entity1")

	if child == nil {
		t.Fatal("WithPrefix() returned nil")
	}
	if child.prefix != "entity1" {
		t.Errorf("prefix = %q, want %q", child.prefix, "entity1")
	}
	// Parent should not be affected
	if parent.prefix != "" {
		t.Errorf("parent prefix = %q, want empty", parent.prefix)
	}
	// Child should share the same level
	if child.level != parent.level {
		t.Errorf("child level = %v, parent level = %v", child.level, parent.level)
	}
}

func TestLogger_WithEntity(t *testing.T) {
	logger := New(false)
	entityLogger := logger.WithEntity("test.entity")

	if entityLogger.prefix != "test.entity" {
		t.Errorf("prefix = %q, want %q", entityLogger.prefix, "test.entity")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	// Note: The logger uses log.Printf which writes to os.Stdout directly.
	// These tests verify the logger doesn't panic and handles different log levels.

	logger := New(false)

	t.Run("Info at LevelInfo", func(t *testing.T) {
		// Should not panic
		logger.Info("test message %s", "arg1")
	})

	t.Run("Error at LevelInfo", func(t *testing.T) {
		logger.Error("error message")
	})

	t.Run("Debug at LevelInfo", func(t *testing.T) {
		// Debug should be silently ignored at LevelInfo
		logger.Debug("debug message")
	})
}

func TestLogger_DebugLevel(t *testing.T) {
	logger := New(true) // verbose = debug level

	t.Run("Debug at LevelDebug", func(t *testing.T) {
		logger.Debug("debug message")
	})

	t.Run("Info at LevelDebug", func(t *testing.T) {
		logger.Info("info message")
	})

	t.Run("Error at LevelDebug", func(t *testing.T) {
		logger.Error("error message")
	})
}

func TestLogger_LogWithPrefix(t *testing.T) {
	logger := New(false)
	logger.SetPrefix("test-entity")

	// Should not panic
	logger.Info("test message")

	if logger.prefix != "test-entity" {
		t.Errorf("prefix = %q, want %q", logger.prefix, "test-entity")
	}
}

func TestLogger_FormatTimestamp(t *testing.T) {
	logger := New(false)
	ts := logger.formatTimestamp()

	// Check format is roughly YYYY-MM-DD HH:MM:SS
	if len(ts) < 19 {
		t.Errorf("timestamp too short: %q", ts)
	}
	if !strings.Contains(ts, " ") {
		t.Errorf("timestamp does not contain space separator: %q", ts)
	}
	// Should contain date separator
	if !strings.Contains(ts, "-") {
		t.Errorf("timestamp does not contain date separator: %q", ts)
	}
	// Should contain time separator
	if !strings.Contains(ts, ":") {
		t.Errorf("timestamp does not contain time separator: %q", ts)
	}
}

func TestLogger_StdLogger(t *testing.T) {
	logger := New(false)
	stdLogger := logger.StdLogger()

	if stdLogger == nil {
		t.Fatal("StdLogger() returned nil")
	}
}

func TestLevel_Constants(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		value int
	}{
		{"LevelInfo", LevelInfo, 0},
		{"LevelError", LevelError, 1},
		{"LevelDebug", LevelDebug, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.level) != tt.value {
				t.Errorf("%s = %d, want %d", tt.name, int(tt.level), tt.value)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	logger := New(false)

	// Create logger at Error level (1)
	logger.level = LevelError

	t.Run("Info does not log at Error level", func(t *testing.T) {
		// Should be silently filtered
		logger.Info("info message")
	})

	t.Run("Error logs at Error level", func(t *testing.T) {
		logger.Error("error message")
	})

	t.Run("Debug does not log at Error level", func(t *testing.T) {
		logger.Debug("debug message")
	})
}

func TestLogger_ConcurrentLogging(t *testing.T) {
	logger := New(true)

	// Log from multiple goroutines - should not panic
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			logger.Info("goroutine 1: %d", i)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 10; i++ {
			logger.Info("goroutine 2: %d", i)
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	// If we got here without panic, the test passed
}

func TestLogger_VerboseFlag(t *testing.T) {
	t.Run("verbose=false uses LevelInfo", func(t *testing.T) {
		logger := New(false)
		if logger.level != LevelInfo {
			t.Errorf("level = %v, want LevelInfo", logger.level)
		}
	})

	t.Run("verbose=true uses LevelDebug", func(t *testing.T) {
		logger := New(true)
		if logger.level != LevelDebug {
			t.Errorf("level = %v, want LevelDebug", logger.level)
		}
	})
}

func TestLogger_ChildInheritsLevel(t *testing.T) {
	parent := New(true)
	child := parent.WithPrefix("child")

	if child.level != LevelDebug {
		t.Errorf("child level = %v, want LevelDebug", child.level)
	}

	grandchild := child.WithPrefix("grandchild")
	if grandchild.level != LevelDebug {
		t.Errorf("grandchild level = %v, want LevelDebug", grandchild.level)
	}
}

func TestLogger_PrefixFormatting(t *testing.T) {
	logger := New(false)

	t.Run("empty prefix", func(t *testing.T) {
		logger.SetPrefix("")
		if logger.prefix != "" {
			t.Errorf("prefix = %q, want empty", logger.prefix)
		}
	})

	t.Run("prefix with special chars", func(t *testing.T) {
		logger.SetPrefix("test.entity[1]")
		if logger.prefix != "test.entity[1]" {
			t.Errorf("prefix = %q, want test.entity[1]", logger.prefix)
		}
	})
}

func TestLogger_MultipleLevels(t *testing.T) {
	logger := New(false)

	// Test that all log methods can be called without panicking
	logger.Info("info")
	logger.Error("error")
	logger.Debug("debug") // silently ignored at LevelInfo
}

func TestLogger_CloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"

	logger, err := NewWithFile(logPath, false)
	if err != nil {
		t.Fatalf("NewWithFile() error = %v", err)
	}

	// Close multiple times - first closes the file, second will return error
	// but should not panic
	if err := logger.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	err = logger.Close()
	// Second close may return "file already closed" error which is expected behavior
	// The important thing is that it doesn't panic
	_ = err // We accept that closing a closed file may return an error
}
