package db

import (
	"context"
	"database/sql"
	"fmt"
)

// MockDB is a mock implementation of the DB interface for testing
type MockDB struct {
	// CloseFunc is called when Close is invoked
	CloseFunc func() error
	// QueryFunc is called when QueryContext is invoked
	QueryFunc func(ctx context.Context, query string, args map[string]interface{}) (*sql.Rows, error)
	// PingFunc is called when Ping is invoked
	PingFunc func(ctx context.Context) error
	// Closed tracks if Close was called
	Closed bool
}

// NewMockDB creates a new MockDB with default no-op functions
func NewMockDB() *MockDB {
	return &MockDB{
		CloseFunc: func() error {
			return nil
		},
		QueryFunc: func(ctx context.Context, query string, args map[string]interface{}) (*sql.Rows, error) {
			return nil, fmt.Errorf("no query result configured")
		},
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}
}

// Close closes the mock database
func (m *MockDB) Close() error {
	m.Closed = true
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// QueryContext executes a query with context and named parameters
func (m *MockDB) QueryContext(ctx context.Context, query string, args map[string]interface{}) (*sql.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args)
	}
	return nil, fmt.Errorf("query not configured")
}

// Ping checks if the database connection is alive
func (m *MockDB) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// MockRowScanner is a mock implementation of RowScanner for testing
type MockRowScanner struct {
	// ColumnNames to return
	ColumnNames []string
	// Rows of data (each row is a slice of string values)
	Rows [][]string
	// Current row index
	currentRow int
	// Error to return on Next/Scan/Err
	NextErr  error
	ScanErr  error
	CloseErr error
	ErrVal   error
}

// NewMockRowScanner creates a new MockRowScanner
func NewMockRowScanner(columns []string, rows [][]string) *MockRowScanner {
	return &MockRowScanner{
		ColumnNames: columns,
		Rows:        rows,
		currentRow:  -1,
	}
}

// Next advances to the next row
func (m *MockRowScanner) Next() bool {
	if m.NextErr != nil {
		return false
	}
	m.currentRow++
	return m.currentRow < len(m.Rows)
}

// Scan copies the column values into the provided destinations
func (m *MockRowScanner) Scan(dest ...interface{}) error {
	if m.ScanErr != nil {
		return m.ScanErr
	}
	if m.currentRow < 0 || m.currentRow >= len(m.Rows) {
		return fmt.Errorf("no current row")
	}
	row := m.Rows[m.currentRow]
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

// Columns returns the column names
func (m *MockRowScanner) Columns() ([]string, error) {
	return m.ColumnNames, nil
}

// Close closes the row scanner
func (m *MockRowScanner) Close() error {
	return m.CloseErr
}

// Err returns any error encountered during row iteration
func (m *MockRowScanner) Err() error {
	if m.ErrVal != nil {
		return m.ErrVal
	}
	if m.NextErr != nil {
		return m.NextErr
	}
	return nil
}

// AddRow adds a row to the mock scanner
func (m *MockRowScanner) AddRow(row ...string) {
	m.Rows = append(m.Rows, row)
}

// SetNextError sets an error to be returned by the next Next() call
func (m *MockRowScanner) SetNextError(err error) {
	m.NextErr = err
}

// SetScanError sets an error to be returned by Scan() calls
func (m *MockRowScanner) SetScanError(err error) {
	m.ScanErr = err
}
