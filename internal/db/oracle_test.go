package db

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := &Config{
		User:           "test",
		Password:       "test",
		Host:           "localhost",
		Port:           1521,
		Service:        "TEST",
		ConnectTimeout: 0,
	}

	db := New(cfg)
	if db == nil {
		t.Fatal("New() returned nil")
	}

	if db.conn != nil {
		t.Error("New() should not connect immediately")
	}
}

func TestConnectString_Empty(t *testing.T) {
	_, err := ConnectString(context.Background(), "", "", "", 0)
	if err == nil {
		t.Error("expected error for empty connection string, got nil")
	}
}

func TestMockDB(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		mock := NewMockDB()
		closeCalled := false
		mock.CloseFunc = func() error {
			closeCalled = true
			return nil
		}

		err := mock.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !closeCalled {
			t.Error("CloseFunc was not called")
		}
		if !mock.Closed {
			t.Error("Closed flag not set")
		}
	})

	t.Run("Ping", func(t *testing.T) {
		mock := NewMockDB()
		pingCalled := false
		mock.PingFunc = func(ctx context.Context) error {
			pingCalled = true
			return nil
		}

		err := mock.Ping(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !pingCalled {
			t.Error("PingFunc was not called")
		}
	})

	t.Run("QueryContext error when not configured", func(t *testing.T) {
		mock := NewMockDB()
		// Reset QueryFunc to return error by default
		mock.QueryFunc = nil

		_, err := mock.QueryContext(context.Background(), "SELECT 1", nil)
		if err == nil {
			t.Error("expected error when QueryFunc not configured, got nil")
		}
	})
}

func TestMockRowScanner(t *testing.T) {
	t.Run("basic scanning", func(t *testing.T) {
		columns := []string{"id", "name"}
		rows := [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		}

		scanner := NewMockRowScanner(columns, rows)

		// Get columns
		cols, err := scanner.Columns()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(cols) != 2 {
			t.Errorf("got %d columns, want 2", len(cols))
		}

		// First row
		if !scanner.Next() {
			t.Fatal("Next() returned false, want true")
		}
		var id1, name1 string
		err = scanner.Scan(&id1, &name1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if id1 != "1" {
			t.Errorf("got id %q, want %q", id1, "1")
		}
		if name1 != "Alice" {
			t.Errorf("got name %q, want %q", name1, "Alice")
		}

		// Second row
		if !scanner.Next() {
			t.Fatal("Next() returned false, want true")
		}
		var id2, name2 string
		err = scanner.Scan(&id2, &name2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if id2 != "2" {
			t.Errorf("got id %q, want %q", id2, "2")
		}
		if name2 != "Bob" {
			t.Errorf("got name %q, want %q", name2, "Bob")
		}

		// No more rows
		if scanner.Next() {
			t.Error("Next() returned true, want false")
		}
	})

	t.Run("Next error", func(t *testing.T) {
		scanner := NewMockRowScanner([]string{"id"}, [][]string{})
		scanner.SetNextError(testErr("next error"))

		if scanner.Next() {
			t.Error("Next() returned true, want false")
		}
		if scanner.Err() == nil {
			t.Error("Err() returned nil, want error")
		}
	})

	t.Run("Scan error", func(t *testing.T) {
		scanner := NewMockRowScanner([]string{"id"}, [][]string{{"1"}})
		scanner.SetScanError(testErr("scan error"))

		if !scanner.Next() {
			t.Fatal("Next() returned false, want true")
		}
		var id string
		if scanner.Scan(&id) == nil {
			t.Error("Scan() returned nil, want error")
		}
	})
}

func TestMockRowScanner_AddRow(t *testing.T) {
	scanner := NewMockRowScanner([]string{"id", "name"}, [][]string{})
	scanner.AddRow("1", "Alice")
	scanner.AddRow("2", "Bob")

	if !scanner.Next() {
		t.Fatal("Next() returned false, want true")
	}
	var id, name string
	scanner.Scan(&id, &name)
	if id != "1" {
		t.Errorf("got id %q, want %q", id, "1")
	}
	if name != "Alice" {
		t.Errorf("got name %q, want %q", name, "Alice")
	}
}

type testErr string

func (e testErr) Error() string {
	return string(e)
}
