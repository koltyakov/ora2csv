package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	go_ora "github.com/sijms/go-ora/v2"
)

// DB defines the interface for database operations
type DB interface {
	Close() error
	QueryContext(ctx context.Context, query string, args map[string]interface{}) (*sql.Rows, error)
	Ping(ctx context.Context) error
}

// OracleDB implements the DB interface using go-ora
type OracleDB struct {
	conn *sql.DB
}

// Config holds database connection configuration
type Config struct {
	User           string
	Password       string
	Host           string
	Port           int
	Service        string
	ConnectTimeout time.Duration
}

// New creates a new OracleDB instance
func New(cfg *Config) *OracleDB {
	return &OracleDB{}
}

// ConnectString creates a new OracleDB and connects using a connection string
// The connection string should be in format: user/password@host:port/service
// or: user/password@//host:port/service
func ConnectString(ctx context.Context, connString, user, password string, timeout time.Duration) (*OracleDB, error) {
	// If connString is empty, we can't connect
	if connString == "" {
		return nil, fmt.Errorf("empty connection string")
	}

	// Create connector from connection string
	connector := go_ora.NewConnector(connString)

	// Open database using the connector
	sqlDB := sql.OpenDB(connector)
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &OracleDB{
		conn: sqlDB,
	}, nil
}

// Close closes the database connection
func (o *OracleDB) Close() error {
	if o.conn != nil {
		return o.conn.Close()
	}
	return nil
}

// QueryContext executes a query with context and named parameters
func (o *OracleDB) QueryContext(ctx context.Context, query string, args map[string]interface{}) (*sql.Rows, error) {
	// go-ora v2 supports named parameters using :param syntax
	// We need to convert the args map to the format expected by go-ora
	return o.conn.QueryContext(ctx, query, argsToSlice(args)...)
}

// Ping checks if the database connection is alive
func (o *OracleDB) Ping(ctx context.Context) error {
	if o.conn == nil {
		return fmt.Errorf("database not connected")
	}
	return o.conn.PingContext(ctx)
}

// argsToSlice converts a map of named arguments to a slice for go-ora
// go-ora expects parameters in the order they appear in the query
func argsToSlice(args map[string]interface{}) []interface{} {
	if len(args) == 0 {
		return nil
	}

	// Common bind variables used in our SQL
	// We return them in a predictable order
	result := make([]interface{}, 0, 2)
	if startDate, ok := args["startDate"]; ok {
		result = append(result, sql.Named("startDate", startDate))
	}
	if tillDate, ok := args["tillDate"]; ok {
		result = append(result, sql.Named("tillDate", tillDate))
	}

	// Add any other parameters
	for k, v := range args {
		if k != "startDate" && k != "tillDate" {
			result = append(result, sql.Named(k, v))
		}
	}

	return result
}
