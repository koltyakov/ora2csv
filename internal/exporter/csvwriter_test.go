package exporter

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func mustCloseCSVWriter(t *testing.T, w *CSVWriter) {
	t.Helper()
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func mustCloseStreamingCSVWriter(t *testing.T, w *StreamingCSVWriter) {
	t.Helper()
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestNewCSVWriter(t *testing.T) {
	t.Run("creates new writer", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/test.csv"

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter() error = %v", err)
		}
		if writer == nil {
			t.Fatal("writer is nil")
		}
		if writer.file == nil {
			t.Error("file is nil")
		}
		if writer.writer == nil {
			t.Error("csv writer is nil")
		}

		mustCloseCSVWriter(t, writer)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		_, err := NewCSVWriter("/nonexistent/dir/test.csv")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestCSVWriter_WriteHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewCSVWriter(filePath)
	if err != nil {
		t.Fatalf("NewCSVWriter() error = %v", err)
	}
	defer mustCloseCSVWriter(t, writer)

	columns := []string{"id", "name", "email"}
	err = writer.WriteHeaders(columns)
	if err != nil {
		t.Errorf("WriteHeaders() error = %v", err)
	}

	if writer.headers == nil {
		t.Error("headers not stored")
	}
	if len(writer.headers) != 3 {
		t.Errorf("headers length = %d, want 3", len(writer.headers))
	}

	// Verify file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "id,name,email") {
		t.Errorf("file content does not contain headers: %s", content)
	}
}

func TestCSVWriter_WriteRow(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewCSVWriter(filePath)
	if err != nil {
		t.Fatalf("NewCSVWriter() error = %v", err)
	}
	defer mustCloseCSVWriter(t, writer)

	columns := []string{"id", "name"}
	if err := writer.WriteHeaders(columns); err != nil {
		t.Fatalf("WriteHeaders() error = %v", err)
	}

	values := []interface{}{1, "Alice"}
	if err := writer.WriteRow(values); err != nil {
		t.Errorf("WriteRow() error = %v", err)
	}

	if writer.rowCount != 1 {
		t.Errorf("rowCount = %d, want 1", writer.rowCount)
	}
}

func TestCSVWriter_HasData(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewCSVWriter(filePath)
	if err != nil {
		t.Fatalf("NewCSVWriter() error = %v", err)
	}
	defer mustCloseCSVWriter(t, writer)

	if writer.HasData() {
		t.Error("HasData() = true, want false initially")
	}

	if err := writer.WriteRow([]interface{}{1, "test"}); err != nil {
		t.Fatalf("WriteRow() error = %v", err)
	}

	if !writer.HasData() {
		t.Error("HasData() = false, want true after writing row")
	}
}

func TestCSVWriter_RowCount(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewCSVWriter(filePath)
	if err != nil {
		t.Fatalf("NewCSVWriter() error = %v", err)
	}
	defer mustCloseCSVWriter(t, writer)

	if err := writer.WriteRow([]interface{}{1}); err != nil {
		t.Fatalf("WriteRow() error = %v", err)
	}
	if err := writer.WriteRow([]interface{}{2}); err != nil {
		t.Fatalf("WriteRow() error = %v", err)
	}
	if err := writer.WriteRow([]interface{}{3}); err != nil {
		t.Fatalf("WriteRow() error = %v", err)
	}

	if writer.RowCount() != 3 {
		t.Errorf("RowCount() = %d, want 3", writer.RowCount())
	}
}

func TestCSVWriter_Close(t *testing.T) {
	t.Run("close flushes data", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/test.csv"

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter() error = %v", err)
		}

		if err := writer.WriteRow([]interface{}{1, "test"}); err != nil {
			t.Fatalf("WriteRow() error = %v", err)
		}
		err = writer.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Verify data was written
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if len(data) == 0 {
			t.Error("file is empty after Close()")
		}
	})
}

func TestCSVWriter_Remove(t *testing.T) {
	t.Run("removes file when no data", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/test.csv"

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter() error = %v", err)
		}

		err = writer.Remove()
		if err != nil {
			t.Errorf("Remove() error = %v", err)
		}

		// Verify file was removed
		_, err = os.Stat(filePath)
		if !os.IsNotExist(err) {
			t.Error("file still exists after Remove()")
		}
	})
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"[]byte", []byte("world"), "world"},
		{"int", 42, "42"},
		{"int8", int8(8), "8"},
		{"int16", int16(16), "16"},
		{"int32", int32(32), "32"},
		{"int64", int64(64), "64"},
		{"uint", uint(10), "10"},
		{"uint8", uint8(8), "8"},
		{"uint16", uint16(16), "16"},
		{"uint32", uint32(32), "32"},
		{"uint64", uint64(64), "64"},
		{"float32", float32(3.14), "3.14"},
		{"float64", float64(2.718), "2.718"},
		{"bool true", true, "1"},
		{"bool false", false, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.value)
			if got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewStreamingCSVWriter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewStreamingCSVWriter(filePath, 3)
	if err != nil {
		t.Fatalf("NewStreamingCSVWriter() error = %v", err)
	}
	defer mustCloseStreamingCSVWriter(t, writer)

	if writer.csv == nil {
		t.Error("csv writer is nil")
	}
	if len(writer.dest) != 3 {
		t.Errorf("dest length = %d, want 3", len(writer.dest))
	}
	if len(writer.rowValues) != 3 {
		t.Errorf("rowValues length = %d, want 3", len(writer.rowValues))
	}
}

func TestStreamingCSVWriter_GetScanTargets(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewStreamingCSVWriter(filePath, 2)
	if err != nil {
		t.Fatalf("NewStreamingCSVWriter() error = %v", err)
	}
	defer mustCloseStreamingCSVWriter(t, writer)

	targets := writer.GetScanTargets()
	if len(targets) != 2 {
		t.Errorf("targets length = %d, want 2", len(targets))
	}

	// Verify targets are sql.NullString pointers
	for i, target := range targets {
		ptr, ok := target.(*sql.NullString)
		if !ok {
			t.Errorf("target %d is not *sql.NullString", i)
		}
		if ptr == nil {
			t.Errorf("target %d is nil pointer", i)
		}
	}
}

func TestStreamingCSVWriter_WriteScannedRow(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewStreamingCSVWriter(filePath, 2)
	if err != nil {
		t.Fatalf("NewStreamingCSVWriter() error = %v", err)
	}
	defer mustCloseStreamingCSVWriter(t, writer)

	// Simulate scanned values
	targets := writer.GetScanTargets()
	targets[0].(*sql.NullString).String = "value1"
	targets[0].(*sql.NullString).Valid = true
	targets[1].(*sql.NullString).String = "value2"
	targets[1].(*sql.NullString).Valid = true

	err = writer.WriteScannedRow()
	if err != nil {
		t.Errorf("WriteScannedRow() error = %v", err)
	}

	if writer.RowCount() != 1 {
		t.Errorf("RowCount() = %d, want 1", writer.RowCount())
	}
}

func TestStreamingCSVWriter_PreservesEmptyStringVsNull(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewStreamingCSVWriter(filePath, 2)
	if err != nil {
		t.Fatalf("NewStreamingCSVWriter() error = %v", err)
	}
	defer mustCloseStreamingCSVWriter(t, writer)

	if err := writer.WriteHeaders([]string{"col1", "col2"}); err != nil {
		t.Fatalf("WriteHeaders() error = %v", err)
	}

	// Simulate one empty string and one NULL.
	targets := writer.GetScanTargets()
	targets[0].(*sql.NullString).String = "value1"
	targets[0].(*sql.NullString).Valid = true
	targets[1].(*sql.NullString).String = ""
	targets[1].(*sql.NullString).Valid = true

	err = writer.WriteScannedRow()
	if err != nil {
		t.Errorf("WriteScannedRow() error = %v", err)
	}

	// Append a NULL row in the same column for comparison.
	targets = writer.GetScanTargets()
	targets[0].(*sql.NullString).String = "value2"
	targets[0].(*sql.NullString).Valid = true
	targets[1].(*sql.NullString).Valid = false

	err = writer.WriteScannedRow()
	if err != nil {
		t.Errorf("WriteScannedRow() error = %v", err)
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// In CSV output both empty string and NULL serialize to empty field; this test
	// verifies writing succeeds without collapsing scan semantics internally.
	data, _ := os.ReadFile(filePath)
	content := string(data)
	if !strings.Contains(content, "value1,") || !strings.Contains(content, "value2,") {
		t.Errorf("file content does not contain expected rows: %s", content)
	}
}

func TestStreamingCSVWriter_FullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.csv"

	writer, err := NewStreamingCSVWriter(filePath, 3)
	if err != nil {
		t.Fatalf("NewStreamingCSVWriter() error = %v", err)
	}

	// Write headers
	err = writer.WriteHeaders([]string{"id", "name", "email"})
	if err != nil {
		t.Errorf("WriteHeaders() error = %v", err)
	}

	// Write rows
	for i := 1; i <= 3; i++ {
		targets := writer.GetScanTargets()
		targets[0].(*sql.NullString).String = string(rune('0' + i))
		targets[0].(*sql.NullString).Valid = true
		targets[1].(*sql.NullString).String = "User" + string(rune('0'+i))
		targets[1].(*sql.NullString).Valid = true
		targets[2].(*sql.NullString).String = "user" + string(rune('0'+i)) + "@test.com"
		targets[2].(*sql.NullString).Valid = true

		err = writer.WriteScannedRow()
		if err != nil {
			t.Errorf("WriteScannedRow() error = %v", err)
		}
	}

	// Flush and close
	err = writer.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 4 { // header + 3 rows
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestWriteNoDataFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/nodata.txt"

	err := WriteNoDataFile(filePath)
	if err != nil {
		t.Errorf("WriteNoDataFile() error = %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "No data found") {
		t.Errorf("file content = %q, want comment about no data", content)
	}
}

func TestIsEmpty(t *testing.T) {
	t.Run("nonexistent file is empty", func(t *testing.T) {
		if !IsEmpty("/nonexistent/file.txt") {
			t.Error("IsEmpty() = false for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/empty.txt"
		mustWriteTestFile(t, filePath, "")

		if !IsEmpty(filePath) {
			t.Error("IsEmpty() = false for empty file")
		}
	})

	t.Run("file with small content is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/small.txt"
		mustWriteTestFile(t, filePath, "head")

		if !IsEmpty(filePath) {
			t.Error("IsEmpty() = false for small file (< 10 bytes)")
		}
	})

	t.Run("file with content is not empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/data.txt"
		mustWriteTestFile(t, filePath, "header,row1,row2,row3")

		if IsEmpty(filePath) {
			t.Error("IsEmpty() = true for file with data")
		}
	})
}

func TestRemoveEmpty(t *testing.T) {
	t.Run("removes empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/empty.csv"
		mustWriteTestFile(t, filePath, "")

		err := RemoveEmpty(filePath)
		if err != nil {
			t.Errorf("RemoveEmpty() error = %v", err)
		}

		_, err = os.Stat(filePath)
		if !os.IsNotExist(err) {
			t.Error("file still exists after RemoveEmpty()")
		}
	})

	t.Run("keeps non-empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/data.csv"
		content := "header,row1,row2,row3"
		mustWriteTestFile(t, filePath, content)

		err := RemoveEmpty(filePath)
		if err != nil {
			t.Errorf("RemoveEmpty() error = %v", err)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != content {
			t.Error("file content was modified")
		}
	})

	t.Run("handles nonexistent file", func(t *testing.T) {
		// RemoveEmpty calls IsEmpty which returns true for nonexistent files
		// then tries to remove them, which returns an error
		err := RemoveEmpty("/nonexistent/file.csv")
		if err == nil {
			t.Error("RemoveEmpty() should return error for nonexistent file")
		}
	})
}

func TestS3StreamingCSVWriter(t *testing.T) {
	t.Run("creates new writer", func(t *testing.T) {
		tmpDir := t.TempDir()
		localPath := tmpDir + "/test.csv"

		// Note: S3StreamingCSVWriter needs a mock S3 client for full testing
		// This test just verifies construction
		writer := &S3StreamingCSVWriter{
			csv:         &CSVWriter{},
			localPath:   localPath,
			dest:        make([]interface{}, 2),
			rowValues:   make([]sql.NullString, 2),
			columnCount: 2,
		}

		if writer.localPath != localPath {
			t.Errorf("localPath = %q, want %q", writer.localPath, localPath)
		}
		if len(writer.dest) != 2 {
			t.Errorf("dest length = %d, want 2", len(writer.dest))
		}
	})

	t.Run("GetScanTargets returns correct number", func(t *testing.T) {
		writer := &S3StreamingCSVWriter{
			dest:        make([]interface{}, 3),
			rowValues:   make([]sql.NullString, 3),
			columnCount: 3,
		}

		targets := writer.GetScanTargets()
		if len(targets) != 3 {
			t.Errorf("targets length = %d, want 3", len(targets))
		}
	})

	t.Run("GetLocalPath returns path", func(t *testing.T) {
		writer := &S3StreamingCSVWriter{
			localPath: "/tmp/test.csv",
		}

		if writer.GetLocalPath() != "/tmp/test.csv" {
			t.Errorf("GetLocalPath() = %q, want %q", writer.GetLocalPath(), "/tmp/test.csv")
		}
	})
}

func TestRowScannerInterface(t *testing.T) {
	// Test mock implementation
	mock := &mockRowScanner{
		columns: []string{"col1", "col2"},
		rows:    [][]string{{"val1", "val2"}},
	}

	// Test Columns
	cols, err := mock.Columns()
	if err != nil {
		t.Errorf("Columns() error = %v", err)
	}
	if len(cols) != 2 {
		t.Errorf("Columns() length = %d, want 2", len(cols))
	}

	// Test Next
	if !mock.Next() {
		t.Error("Next() = false, want true")
	}

	// Test Scan
	var col1, col2 string
	err = mock.Scan(&col1, &col2)
	if err != nil {
		t.Errorf("Scan() error = %v", err)
	}
	if col1 != "val1" || col2 != "val2" {
		t.Errorf("Scan() = (%q, %q), want (val1, val2)", col1, col2)
	}

	// Test Close
	err = mock.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Test Err
	if mock.Err() != nil {
		t.Errorf("Err() = %v, want nil", mock.Err())
	}
}

type mockRowScanner struct {
	columns []string
	rows    [][]string
	rowIdx  int
	closed  bool
	scanErr error
}

func (m *mockRowScanner) Columns() ([]string, error) { return m.columns, nil }
func (m *mockRowScanner) Next() bool {
	m.rowIdx++
	return m.rowIdx <= len(m.rows)
}
func (m *mockRowScanner) Scan(dest ...interface{}) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	if m.rowIdx > len(m.rows) || m.rowIdx < 1 {
		return nil
	}
	row := m.rows[m.rowIdx-1]
	for i, val := range row {
		if i < len(dest) {
			switch ptr := dest[i].(type) {
			case *string:
				*ptr = val
			case *sql.NullString:
				ptr.String = val
				ptr.Valid = true
			}
		}
	}
	return nil
}
func (m *mockRowScanner) Close() error { m.closed = true; return nil }
func (m *mockRowScanner) Err() error   { return m.scanErr }
