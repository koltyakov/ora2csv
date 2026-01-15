package exporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koltyakov/ora2csv/internal/config"
	"github.com/koltyakov/ora2csv/internal/db"
	"github.com/koltyakov/ora2csv/internal/logging"
	"github.com/koltyakov/ora2csv/internal/state"
	"github.com/koltyakov/ora2csv/internal/storage"
	"github.com/koltyakov/ora2csv/pkg/types"
)

// Exporter handles the main export orchestration
type Exporter struct {
	cfg    *config.Config
	db     *db.OracleDB
	st     *state.File
	logger *logging.Logger
	s3     *storage.S3Client
}

// New creates a new Exporter
func New(cfg *config.Config, database *db.OracleDB, st *state.File, logger *logging.Logger, s3 *storage.S3Client) *Exporter {
	return &Exporter{
		cfg:    cfg,
		db:     database,
		st:     st,
		logger: logger,
		s3:     s3,
	}
}

// Run executes the export process for all active entities
func (e *Exporter) Run(ctx context.Context) (*types.ExportResult, error) {
	startTime := time.Now()
	result := &types.ExportResult{
		Results: make([]types.EntityResult, 0),
	}

	e.logger.Info("Starting data export process")
	e.logger.Info("Total entities: %d, Active: %d", e.st.TotalCount(), e.st.ActiveCount())

	// Capture till date once for all entities (use UTC to avoid timezone issues)
	tillDate := time.Now().UTC()
	tillDateStr := tillDate.Format("2006-01-02T15:04:05")
	e.logger.Info("Using till date for all entities: %s", tillDateStr)

	// Process each active entity
	for _, entity := range e.st.GetActiveEntities() {
		entityResult := e.processEntity(ctx, entity, tillDate, tillDateStr)
		result.Results = append(result.Results, entityResult)
		result.ProcessedCount++

		if entityResult.Success {
			result.SuccessCount++
			// Update state only on success
			if err := e.st.UpdateEntityTimestamp(entity.Entity, tillDateStr); err != nil {
				e.logger.Error("Failed to update state for %s: %v", entity.Entity, err)
				result.TotalEntities = e.st.TotalCount()
				result.Duration = time.Since(startTime)
				return result, fmt.Errorf("failed to update state for %s: %w", entity.Entity, err)
			}
		} else {
			result.FailedCount++
			result.TotalEntities = e.st.TotalCount()
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("entity %s failed: %w", entity.Entity, entityResult.Error)
		}
	}

	result.TotalEntities = e.st.TotalCount()
	result.Duration = time.Since(startTime)

	return result, nil
}

// processEntity handles the export of a single entity
func (e *Exporter) processEntity(ctx context.Context, entity types.EntityState, tillDate time.Time, tillDateStr string) types.EntityResult {
	startTime := time.Now()
	log := e.logger.WithEntity(entity.Entity)

	log.Info("Processing entity: %s (active: %t)", entity.Entity, entity.Active)

	// Determine start date
	startDate, err := e.getStartDate(entity)
	if err != nil {
		log.Error("Failed to determine start date: %v", err)
		return types.EntityResult{
			Entity:   entity.Entity,
			Success:  false,
			Error:    fmt.Errorf("failed to determine start date: %w", err),
			Duration: time.Since(startTime),
		}
	}
	startDateStr := startDate.Format("2006-01-02T15:04:05")

	log.Info("Start date: %s", startDateStr)

	// Load SQL file
	sqlContent, err := e.loadSQLFile(entity.Entity)
	if err != nil {
		log.Error("Failed to load SQL file: %v", err)
		return types.EntityResult{
			Entity:   entity.Entity,
			Success:  false,
			Error:    fmt.Errorf("failed to load SQL file: %w", err),
			Duration: time.Since(startTime),
		}
	}

	// Generate output filename
	outputFile := e.getOutputPath(entity.Entity, startDateStr)
	log.Info("Output file: %s", outputFile)

	// Create export directory
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		log.Error("Failed to create output directory: %v", err)
		return types.EntityResult{
			Entity:   entity.Entity,
			Success:  false,
			Error:    fmt.Errorf("failed to create output directory: %w", err),
			Duration: time.Since(startTime),
		}
	}

	// Execute query and stream to CSV
	rowCount, err := e.executeQueryToCSV(ctx, sqlContent, startDateStr, tillDateStr, outputFile, log)
	if err != nil {
		log.Error("Failed to execute query: %v", err)
		return types.EntityResult{
			Entity:   entity.Entity,
			Success:  false,
			Error:    err,
			Duration: time.Since(startTime),
		}
	}

	if rowCount == 0 {
		log.Info("No data rows found for entity: %s - skipping CSV creation", entity.Entity)
		// Still update state since query succeeded
		return types.EntityResult{
			Entity:   entity.Entity,
			Success:  true,
			RowCount: 0,
			Duration: time.Since(startTime),
		}
	}

	log.Info("Exported %d rows to: %s", rowCount, outputFile)

	return types.EntityResult{
		Entity:   entity.Entity,
		Success:  true,
		RowCount: rowCount,
		FilePath: outputFile,
		Duration: time.Since(startTime),
	}
}

// getStartDate determines the start date for an entity
func (e *Exporter) getStartDate(entity types.EntityState) (time.Time, error) {
	lastRunTime, err := entity.GetLastRunTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse lastRunTime: %w", err)
	}

	// If no last run time, use default days back (UTC to avoid timezone issues)
	if lastRunTime.IsZero() {
		return time.Now().UTC().AddDate(0, 0, -e.cfg.DefaultDaysBack), nil
	}

	return lastRunTime, nil
}

// loadSQLFile reads the SQL file for an entity
func (e *Exporter) loadSQLFile(entityName string) (string, error) {
	sqlPath := e.st.GetSQLPath(e.cfg.SQLDir, entityName)

	content, err := os.ReadFile(sqlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SQL file %s: %w", sqlPath, err)
	}

	return string(content), nil
}

// getOutputPath generates the output file path for an entity
func (e *Exporter) getOutputPath(entityName, startDate string) string {
	// Replace colons with dashes for filename (matches bash script)
	safeDate := strings.ReplaceAll(startDate, ":", "-")
	filename := fmt.Sprintf("%s__%s.csv", entityName, safeDate)
	return filepath.Join(e.cfg.ExportDir, filename)
}

// executeQueryToCSV executes a query and streams results to CSV
func (e *Exporter) executeQueryToCSV(ctx context.Context, sqlContent, startDate, tillDate, outputPath string, log *logging.Logger) (int, error) {
	// Prepare query parameters
	params := map[string]interface{}{
		"startDate": startDate,
		"tillDate":  tillDate,
	}

	// Execute query
	rows, err := e.db.QueryContext(ctx, sqlContent, params)
	if err != nil {
		return 0, fmt.Errorf("query execution failed: %w", err)
	}

	// Get column count
	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		return 0, fmt.Errorf("failed to get columns: %w", err)
	}

	// Create the appropriate CSV writer based on S3 configuration
	var writer csvWriter
	if e.s3 != nil && e.cfg.S3.Bucket != "" {
		// Generate S3 key from output path
		safeDate := strings.ReplaceAll(startDate, ":", "-")
		entityName := filepath.Base(outputPath)
		entityName = strings.TrimSuffix(entityName, filepath.Ext(entityName))
		entityName = strings.Split(entityName, "__")[0]
		s3Key := e.cfg.S3.Key(fmt.Sprintf("%s/%s__%s.csv", entityName, entityName, safeDate))

		log.Info("Streaming to S3: %s", s3Key)

		// Create S3 streaming writer
		w, err := NewS3StreamingCSVWriter(e.s3, s3Key, outputPath, len(columns))
		if err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to create S3 CSV writer: %w", err)
		}
		defer w.Close()
		writer = w
	} else {
		// Create local file writer
		w, err := NewStreamingCSVWriter(outputPath, len(columns))
		if err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to create CSV writer: %w", err)
		}
		defer w.Close()
		writer = w
	}

	// Write headers
	if err := writer.WriteHeaders(columns); err != nil {
		return 0, fmt.Errorf("failed to write headers: %w", err)
	}

	// Stream rows
	scanTargets := writer.GetScanTargets()
	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return 0, fmt.Errorf("failed to scan row: %w", err)
		}
		if err := writer.WriteScannedRow(); err != nil {
			return 0, fmt.Errorf("failed to write row: %w", err)
		}
		rowCount++

		// Log progress for large exports
		if rowCount%10000 == 0 {
			log.Debug("Progress: %d rows", rowCount)
		}
	}

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("row iteration error: %w", err)
	}

	// Final flush
	if err := writer.Flush(); err != nil {
		return 0, fmt.Errorf("failed to flush writer: %w", err)
	}

	// If no data rows, remove the file
	if rowCount == 0 {
		writer.Remove()
	}

	return rowCount, nil
}

// csvWriter is the interface for both StreamingCSVWriter and S3StreamingCSVWriter
type csvWriter interface {
	WriteHeaders(columns []string) error
	GetScanTargets() []interface{}
	WriteScannedRow() error
	Flush() error
	Remove() error
	Close() error
}

// Validate validates configuration and SQL files
func Validate(cfg *config.Config, st *state.File, testDB bool) error {
	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Validate SQL files
	if err := st.ValidateSQLFiles(cfg.SQLDir); err != nil {
		return fmt.Errorf("SQL file validation failed: %w", err)
	}

	// Test database connection if requested
	if testDB {
		connStr := cfg.ConnectionString()
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
		defer cancel()

		database, err := db.ConnectString(ctx, connStr, "", "", cfg.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("database connection failed: %w", err)
		}
		defer database.Close()

		if err := database.Ping(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
	}

	return nil
}
