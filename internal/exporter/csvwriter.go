package exporter

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// CSVWriter handles streaming CSV encoding.
type CSVWriter struct {
	writer    *csv.Writer
	file      *os.File
	headers   []string
	rowCount  int
	nullValue string
}

// NewCSVWriter creates a new CSVWriter for the given file path
func NewCSVWriter(filePath string) (*CSVWriter, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return newCSVWriter(file, ""), nil
}

func newCSVWriter(file *os.File, nullValue string) *CSVWriter {
	writer := csv.NewWriter(file)
	// Use Unix line endings (LF)
	writer.UseCRLF = false

	return &CSVWriter{
		writer:    writer,
		file:      file,
		nullValue: nullValue,
	}
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
		strValues[i] = formatValueWithNull(v, w.nullValue)
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
// NULL values use the writer's configured marker.
func formatValue(v interface{}) string {
	return formatValueWithNull(v, "")
}

func formatValueWithNull(v interface{}, nullValue string) string {
	if v == nil {
		return nullValue
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
	var errs []error
	if w.writer != nil {
		w.writer.Flush()
		if err := w.writer.Error(); err != nil {
			errs = append(errs, err)
		}
		w.writer = nil
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
		w.file = nil
	}
	return errors.Join(errs...)
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
	w.writer = nil
	if w.file != nil {
		path := w.file.Name()
		if err := w.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return err
		}
		w.file = nil
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
	rowValues []sql.NullString
	finalPath string
	tempPath  string
	state     writerState
}

type writerState uint8

const (
	writerOpen writerState = iota
	writerCommitted
	writerAborted
)

// NewStreamingCSVWriter creates a writer optimized for streaming database rows
func NewStreamingCSVWriter(filePath string, columnCount int) (*StreamingCSVWriter, error) {
	return NewStreamingCSVWriterWithNullValue(filePath, columnCount, "")
}

// NewStreamingCSVWriterWithNullValue creates a transactional writer with an explicit NULL marker.
func NewStreamingCSVWriterWithNullValue(filePath string, columnCount int, nullValue string) (*StreamingCSVWriter, error) {
	file, err := os.CreateTemp(filepath.Dir(filePath), "."+filepath.Base(filePath)+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary CSV file: %w", err)
	}

	return &StreamingCSVWriter{
		csv:       newCSVWriter(file, nullValue),
		dest:      make([]interface{}, columnCount),
		rowValues: make([]sql.NullString, columnCount),
		finalPath: filePath,
		tempPath:  file.Name(),
		state:     writerOpen,
	}, nil
}

// GetScanTargets returns a slice of interface{} pointers for sql.Rows.Scan
func (w *StreamingCSVWriter) GetScanTargets() []interface{} {
	for i := range w.dest {
		w.rowValues[i] = sql.NullString{}
		w.dest[i] = &w.rowValues[i]
	}
	return w.dest
}

// WriteScannedRow writes the most recently scanned row
func (w *StreamingCSVWriter) WriteScannedRow() error {
	// Convert scanned values preserving the NULL vs empty-string distinction.
	values := make([]interface{}, len(w.rowValues))
	for i, v := range w.rowValues {
		if !v.Valid {
			values[i] = nil
		} else {
			values[i] = v.String
		}
	}
	return w.csv.WriteRow(values)
}

// WriteHeaders writes the header row
func (w *StreamingCSVWriter) WriteHeaders(columns []string) error {
	return w.csv.WriteHeaders(columns)
}

// Flush flushes buffered data
func (w *StreamingCSVWriter) Flush() error {
	return w.csv.Flush()
}

// RowCount returns the number of rows written
func (w *StreamingCSVWriter) RowCount() int {
	return w.csv.RowCount()
}

// Commit flushes and atomically publishes the completed CSV.
func (w *StreamingCSVWriter) Commit(ctx context.Context) error {
	if w.state == writerCommitted {
		return nil
	}
	if w.state == writerAborted {
		return fmt.Errorf("CSV writer was aborted")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.csv.Flush(); err != nil {
		return err
	}
	if w.csv.file != nil {
		if err := w.csv.file.Sync(); err != nil {
			return err
		}
	}
	if err := w.csv.Close(); err != nil {
		return err
	}
	if err := os.Rename(w.tempPath, w.finalPath); err != nil {
		return fmt.Errorf("failed to publish CSV: %w", err)
	}
	w.state = writerCommitted
	return nil
}

// Abort closes and removes an incomplete CSV without touching the destination.
func (w *StreamingCSVWriter) Abort() error {
	if w.state == writerCommitted || w.state == writerAborted {
		return nil
	}
	w.state = writerAborted
	closeErr := w.csv.Close()
	removeErr := os.Remove(w.tempPath)
	if os.IsNotExist(removeErr) {
		removeErr = nil
	}
	return errors.Join(closeErr, removeErr)
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
func StreamFromRows(writer *StreamingCSVWriter, rows RowScanner) (retErr error) {
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close rows: %w", err))
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			// Close errors do not affect emptiness check here.
			_ = err
		}
	}()

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

type streamUploader interface {
	UploadStream(context.Context, string, io.Reader) error
}

// S3StreamingCSVWriter stages CSV data locally, then uploads it on commit.
type S3StreamingCSVWriter struct {
	writer    *StreamingCSVWriter
	uploader  streamUploader
	s3Key     string
	localPath string
	finalized bool
	commitErr error
	aborted   bool
}

// NewS3StreamingCSVWriter creates a writer that stages data locally for S3.
// The completed data is published locally, then uploaded to S3 on Commit.
func NewS3StreamingCSVWriter(uploader streamUploader, s3Key, localPath string, columnCount int) (*S3StreamingCSVWriter, error) {
	return NewS3StreamingCSVWriterWithNullValue(uploader, s3Key, localPath, columnCount, "")
}

// NewS3StreamingCSVWriterWithNullValue creates an S3 writer with an explicit NULL marker.
func NewS3StreamingCSVWriterWithNullValue(uploader streamUploader, s3Key, localPath string, columnCount int, nullValue string) (*S3StreamingCSVWriter, error) {
	writer, err := NewStreamingCSVWriterWithNullValue(localPath, columnCount, nullValue)
	if err != nil {
		return nil, err
	}

	return &S3StreamingCSVWriter{
		writer:    writer,
		uploader:  uploader,
		s3Key:     s3Key,
		localPath: localPath,
	}, nil
}

// GetScanTargets returns a slice of interface{} pointers for sql.Rows.Scan
func (w *S3StreamingCSVWriter) GetScanTargets() []interface{} {
	return w.writer.GetScanTargets()
}

// WriteScannedRow writes the most recently scanned row
func (w *S3StreamingCSVWriter) WriteScannedRow() error {
	return w.writer.WriteScannedRow()
}

// WriteHeaders writes the header row
func (w *S3StreamingCSVWriter) WriteHeaders(columns []string) error {
	return w.writer.WriteHeaders(columns)
}

// Commit publishes the local fallback, uploads it to S3, and removes it on success.
func (w *S3StreamingCSVWriter) Commit(ctx context.Context) error {
	if w.finalized {
		return w.commitErr
	}
	if w.aborted {
		return fmt.Errorf("CSV writer was aborted")
	}
	if err := w.writer.Commit(ctx); err != nil {
		return err
	}
	w.finalized = true

	file, err := os.Open(w.localPath)
	if err != nil {
		w.commitErr = fmt.Errorf("failed to open file for S3 upload: %w", err)
		return w.commitErr
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close local file %s: %v\n", w.localPath, err)
		}
	}()

	if err := w.uploader.UploadStream(ctx, w.s3Key, file); err != nil {
		// S3 upload failed - keep the local file as fallback
		w.commitErr = fmt.Errorf("S3 upload failed: %w (local file kept at %s)", err, w.localPath)
		return w.commitErr
	}

	// S3 upload succeeded - remove local temp file
	if err := os.Remove(w.localPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove local file %s: %v\n", w.localPath, err)
	}

	return nil
}

// Flush flushes buffered data
func (w *S3StreamingCSVWriter) Flush() error {
	return w.writer.Flush()
}

// RowCount returns the number of rows written
func (w *S3StreamingCSVWriter) RowCount() int {
	return w.writer.RowCount()
}

// Abort removes an incomplete local file and never uploads it.
func (w *S3StreamingCSVWriter) Abort() error {
	if w.finalized || w.aborted {
		return nil
	}
	w.aborted = true
	return w.writer.Abort()
}

// GetLocalPath returns the local fallback path.
func (w *S3StreamingCSVWriter) GetLocalPath() string {
	return w.localPath
}
