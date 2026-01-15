package exporter

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/koltyakov/ora2csv/internal/storage"
)

// CSVWriter handles streaming CSV writing with RFC 4180 compliance
type CSVWriter struct {
	writer   *csv.Writer
	file     *os.File
	headers  []string
	rowCount int
}

// NewCSVWriter creates a new CSVWriter for the given file path
func NewCSVWriter(filePath string) (*CSVWriter, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	writer := csv.NewWriter(file)
	// Use Unix line endings (LF)
	writer.UseCRLF = false

	return &CSVWriter{
		writer: writer,
		file:   file,
	}, nil
}

// WriteHeaders writes the CSV header row
func (w *CSVWriter) WriteHeaders(columns []string) error {
	if err := w.writer.Write(columns); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}
	w.headers = columns
	w.writer.Flush()
	return w.writer.Error()
}

// WriteRow writes a single data row
func (w *CSVWriter) WriteRow(values []interface{}) error {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatValue(v)
	}

	if err := w.writer.Write(strValues); err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}

	w.rowCount++

	// Flush periodically to manage memory
	if w.rowCount%1000 == 0 {
		w.writer.Flush()
		return w.writer.Error()
	}

	return nil
}

// formatValue converts any value to string for CSV output
// NULL values become empty strings
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Flush flushes any buffered data
func (w *CSVWriter) Flush() error {
	w.writer.Flush()
	return w.writer.Error()
}

// Close closes the writer and file
func (w *CSVWriter) Close() error {
	if w.writer != nil {
		w.writer.Flush()
		if err := w.writer.Error(); err != nil {
			return err
		}
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// RowCount returns the number of data rows written (excluding header)
func (w *CSVWriter) RowCount() int {
	return w.rowCount
}

// HasData returns true if any data rows have been written
func (w *CSVWriter) HasData() bool {
	return w.rowCount > 0
}

// Remove removes the file if no data was written
func (w *CSVWriter) Remove() error {
	if w.file != nil {
		w.file.Close()
		path := w.file.Name()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// StreamingCSVWriter is a functional-style writer for streaming from database rows
type StreamingCSVWriter struct {
	csv       *CSVWriter
	dest      []interface{}
	rowValues []string
}

// NewStreamingCSVWriter creates a writer optimized for streaming database rows
func NewStreamingCSVWriter(filePath string, columnCount int) (*StreamingCSVWriter, error) {
	csvWriter, err := NewCSVWriter(filePath)
	if err != nil {
		return nil, err
	}

	return &StreamingCSVWriter{
		csv:       csvWriter,
		dest:      make([]interface{}, columnCount),
		rowValues: make([]string, columnCount),
	}, nil
}

// GetScanTargets returns a slice of interface{} pointers for sql.Rows.Scan
func (w *StreamingCSVWriter) GetScanTargets() []interface{} {
	for i := range w.dest {
		w.rowValues[i] = ""
		w.dest[i] = &w.rowValues[i]
	}
	return w.dest
}

// WriteScannedRow writes the most recently scanned row
func (w *StreamingCSVWriter) WriteScannedRow() error {
	// Convert string pointers to actual string values
	values := make([]interface{}, len(w.rowValues))
	for i, v := range w.rowValues {
		// Check for NULL (represented by nil pointer or empty string from scan)
		if v == "" {
			values[i] = nil
		} else {
			values[i] = v
		}
	}
	return w.csv.WriteRow(values)
}

// WriteHeaders writes the header row
func (w *StreamingCSVWriter) WriteHeaders(columns []string) error {
	return w.csv.WriteHeaders(columns)
}

// Close closes the writer
func (w *StreamingCSVWriter) Close() error {
	return w.csv.Close()
}

// Flush flushes buffered data
func (w *StreamingCSVWriter) Flush() error {
	return w.csv.Flush()
}

// RowCount returns the number of rows written
func (w *StreamingCSVWriter) RowCount() int {
	return w.csv.RowCount()
}

// Remove removes the file if no data was written
func (w *StreamingCSVWriter) Remove() error {
	return w.csv.Remove()
}

// RowScanner is an interface for types that can be scanned to CSV
type RowScanner interface {
	Next() bool
	Scan(dest ...interface{}) error
	Columns() ([]string, error)
	Close() error
	Err() error
}

// StreamFromRows streams data from database rows directly to CSV
func StreamFromRows(writer *StreamingCSVWriter, rows RowScanner) error {
	defer rows.Close()

	// Get column names for header
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Write header
	if err := writer.WriteHeaders(columns); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	// Stream rows
	scanTargets := writer.GetScanTargets()
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if err := writer.WriteScannedRow(); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	// Check for errors from row iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	// Final flush
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return nil
}

// WriteNoDataFile writes a file indicating no data was found
func WriteNoDataFile(filePath string) error {
	return os.WriteFile(filePath, []byte("# No data found for export\n"), 0644)
}

// IsEmpty checks if a file exists and is empty
func IsEmpty(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return true
	}

	// Check file size (less than 10 bytes = only header or empty)
	return stat.Size() < 10
}

// RemoveEmpty removes the file if it's empty or has no data rows
func RemoveEmpty(path string) error {
	if IsEmpty(path) {
		return os.Remove(path)
	}
	return nil
}

// S3StreamingCSVWriter streams CSV data directly to S3 via multipart upload
// Data is buffered to a temp file during writing, then uploaded to S3 on Close()
type S3StreamingCSVWriter struct {
	csv         *CSVWriter
	s3          *storage.S3Client
	s3Key       string
	localPath   string // For temp file during writing
	dest        []interface{}
	rowValues   []string
	columnCount int
}

// NewS3StreamingCSVWriter creates a writer that streams to S3
// The data is written to a temp file first, then uploaded to S3 on Close()
func NewS3StreamingCSVWriter(s3 *storage.S3Client, s3Key, localPath string, columnCount int) (*S3StreamingCSVWriter, error) {
	csvWriter, err := NewCSVWriter(localPath)
	if err != nil {
		return nil, err
	}

	return &S3StreamingCSVWriter{
		csv:         csvWriter,
		s3:          s3,
		s3Key:       s3Key,
		localPath:   localPath,
		dest:        make([]interface{}, columnCount),
		rowValues:   make([]string, columnCount),
		columnCount: columnCount,
	}, nil
}

// GetScanTargets returns a slice of interface{} pointers for sql.Rows.Scan
func (w *S3StreamingCSVWriter) GetScanTargets() []interface{} {
	for i := range w.dest {
		w.rowValues[i] = ""
		w.dest[i] = &w.rowValues[i]
	}
	return w.dest
}

// WriteScannedRow writes the most recently scanned row
func (w *S3StreamingCSVWriter) WriteScannedRow() error {
	values := make([]interface{}, len(w.rowValues))
	for i, v := range w.rowValues {
		if v == "" {
			values[i] = nil
		} else {
			values[i] = v
		}
	}
	return w.csv.WriteRow(values)
}

// WriteHeaders writes the header row
func (w *S3StreamingCSVWriter) WriteHeaders(columns []string) error {
	return w.csv.WriteHeaders(columns)
}

// Close flushes, uploads to S3, and removes the local temp file
func (w *S3StreamingCSVWriter) Close() error {
	// Flush and close the local file
	if err := w.csv.Close(); err != nil {
		return err
	}

	// Upload to S3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Open the file for upload
	file, err := os.Open(w.localPath)
	if err != nil {
		return fmt.Errorf("failed to open file for S3 upload: %w", err)
	}
	defer file.Close()

	// Upload to S3 via multipart upload
	if err := w.s3.UploadStream(ctx, w.s3Key, file); err != nil {
		// S3 upload failed - keep the local file as fallback
		return fmt.Errorf("S3 upload failed: %w (local file kept at %s)", err, w.localPath)
	}

	// S3 upload succeeded - remove local temp file
	if err := os.Remove(w.localPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove local file %s: %v\n", w.localPath, err)
	}

	return nil
}

// Flush flushes buffered data
func (w *S3StreamingCSVWriter) Flush() error {
	return w.csv.Flush()
}

// RowCount returns the number of rows written
func (w *S3StreamingCSVWriter) RowCount() int {
	return w.csv.RowCount()
}

// Remove removes the temp file
func (w *S3StreamingCSVWriter) Remove() error {
	return w.csv.Remove()
}

// GetLocalPath returns the local temp file path
func (w *S3StreamingCSVWriter) GetLocalPath() string {
	return w.localPath
}
