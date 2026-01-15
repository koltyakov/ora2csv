package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/koltyakov/ora2csv/internal/config"
	"github.com/koltyakov/ora2csv/internal/db"
	"github.com/koltyakov/ora2csv/internal/exporter"
	"github.com/koltyakov/ora2csv/internal/logging"
	"github.com/koltyakov/ora2csv/internal/state"
	"github.com/koltyakov/ora2csv/internal/storage"
	"github.com/koltyakov/ora2csv/pkg/types"
)

var (
	// Version is set at build time
	version = "dev"
	// BuildTime is set at build time
	buildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ora2csv",
	Short: "Oracle to CSV exporter with state management",
	Long: `ora2csv exports data from Oracle database to CSV files with incremental sync.
It streams data directly from Oracle to CSV without storing entire exports in memory.`,
	Version: version,
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data from Oracle to CSV",
	Long:  "Export data from Oracle database to CSV files based on state.json configuration",
	RunE:  runExport,
	SilenceUsage:  true, // Don't print usage on error
	SilenceErrors: false, // Still print errors
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and SQL files",
	Long:  "Validate configuration, check SQL files exist, and optionally test database connection",
	RunE:  runValidate,
	SilenceUsage: true, // Don't print usage on error
}

func init() {
	// Common flags
	rootCmd.PersistentFlags().String("db-host", config.DefaultDBHost, "Database host")
	rootCmd.PersistentFlags().Int("db-port", config.DefaultDBPort, "Database port")
	rootCmd.PersistentFlags().String("db-service", config.DefaultDBService, "Database service name")
	rootCmd.PersistentFlags().String("db-user", config.DefaultDBUser, "Database user")
	rootCmd.PersistentFlags().String("state-file", config.DefaultStateFile, "Path to state.json file")
	rootCmd.PersistentFlags().String("sql-dir", config.DefaultSQLDir, "Path to SQL directory")
	rootCmd.PersistentFlags().String("export-dir", config.DefaultExportDir, "Path to export directory")
	rootCmd.PersistentFlags().Int("days-back", config.DefaultDaysBack, "Default days to look back for first run")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Validate without executing")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose logging")
	rootCmd.PersistentFlags().Duration("connect-timeout", config.DefaultConnectTimeoutSecs*time.Second, "Connection timeout")
	rootCmd.PersistentFlags().Duration("query-timeout", config.DefaultQueryTimeoutSecs*time.Second, "Query timeout")

	// S3 flags
	rootCmd.PersistentFlags().String("s3-bucket", "", "S3 bucket name")
	rootCmd.PersistentFlags().String("s3-prefix", "", "S3 key prefix")
	rootCmd.PersistentFlags().String("s3-access-key", "", "S3 access key (for S3-compatible services)")
	rootCmd.PersistentFlags().String("s3-secret-key", "", "S3 secret key (for S3-compatible services)")
	rootCmd.PersistentFlags().String("s3-session-token", "", "S3 session token (for S3-compatible services)")
	rootCmd.PersistentFlags().String("s3-endpoint", "", "S3 endpoint URL (for S3-compatible services like MinIO)")

	// Validate-specific flags
	validateCmd.Flags().Bool("test-connection", false, "Test database connection")
}

func main() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(validateCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// setupContext creates a context with cancellation and signal handling
func setupContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	return ctx
}

// connectDatabase establishes a connection to the Oracle database
func connectDatabase(ctx context.Context, cfg *config.Config) (*db.OracleDB, error) {
	connCtx, connCancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer connCancel()

	database, err := db.ConnectString(
		connCtx,
		cfg.ConnectionString(),
		"", // user and password are already in connection string
		"",
		cfg.ConnectTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return database, nil
}

// executeExport runs the export process
func executeExport(ctx context.Context, cfg *config.Config, database *db.OracleDB, st *state.File, logger *logging.Logger) (*types.ExportResult, error) {
	// Create query timeout context
	queryCtx, queryCancel := context.WithTimeout(ctx, cfg.QueryTimeout)
	defer queryCancel()

	// Initialize S3 client if enabled
	var s3Client *storage.S3Client
	if cfg.S3.Bucket != "" {
		logger.Info("Initializing S3 client (bucket: %s)", cfg.S3.Bucket)
		client, err := storage.NewS3Client(&cfg.S3)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
		}
		s3Client = client
		logger.Info("S3 client initialized")
	}

	// Create and run exporter
	exp := exporter.New(cfg, database, st, logger, s3Client)
	return exp.Run(queryCtx)
}

// printSummary prints the export result summary
func printSummary(result *types.ExportResult, cfg *config.Config, logger *logging.Logger) {
	duration := result.Duration
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60

	logger.Info("==================================================")
	logger.Info("Export completed successfully")
	logger.Info("Total duration: %dm %ds", minutes, seconds)
	logger.Info("Total entities: %d", result.TotalEntities)
	logger.Info("Successfully processed: %d", result.SuccessCount)
	if result.FailedCount > 0 {
		logger.Error("Failed entities: %d", result.FailedCount)
	}
	logger.Info("Skipped (inactive): %d", result.TotalEntities-result.ProcessedCount)
	logger.Info("==================================================")

	// Print per-entity results if verbose
	if cfg.Verbose {
		for _, r := range result.Results {
			if r.Success {
				logger.Info("  ✓ %s: %d rows (%v)", r.Entity, r.RowCount, r.Duration)
			} else {
				logger.Error("  ✗ %s: %v", r.Entity, r.Error)
			}
		}
	}
}

func runExport(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg := config.FromCommand(cmd)

	// Setup context with signal handling
	ctx := setupContext()
	defer func() {
		// Cancel context to ensure goroutines exit
		select {
		case <-ctx.Done():
		default:
		}
	}()

	// Create logger
	logger := logging.New(cfg.Verbose)
	defer logger.Close()

	logger.Info("Starting ora2csv v%s (built: %s)", version, buildTime)

	// Validate configuration (including S3)
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
		return err
	}

	// Initialize S3 client if enabled
	var s3Client *storage.S3Client
	var s3StateKey string
	if cfg.S3.Bucket != "" {
		logger.Info("S3 destination enabled (bucket: %s)", cfg.S3.Bucket)
		client, err := storage.NewS3Client(&cfg.S3)
		if err != nil {
			logger.Error("Failed to initialize S3 client: %v", err)
			return fmt.Errorf("failed to initialize S3 client: %w", err)
		}
		s3Client = client
		s3StateKey = cfg.S3.StateKey()
		logger.Info("S3 client initialized")

		// Check S3 connectivity before starting export
		logger.Info("Checking S3 connectivity...")
		checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := s3Client.CheckConnection(checkCtx); err != nil {
			logger.Error("S3 connectivity check failed: %v", err)
			return fmt.Errorf("S3 connectivity check failed: %w", err)
		}
		checkCancel()
		logger.Info("S3 connectivity verified")
	}

	// Load state file (with S3 sync if enabled)
	st, err := state.Load(cfg.StateFile, s3Client, s3StateKey)
	if err != nil {
		logger.Error("Failed to load state file: %v", err)
		return fmt.Errorf("failed to load state file: %w", err)
	}

	logger.Info("Loaded state file: %s (%d entities, %d active)",
		cfg.StateFile, st.TotalCount(), st.ActiveCount())

	// Dry run mode
	if cfg.DryRun {
		logger.Info("Dry run mode - validating configuration only")
		if err := exporter.Validate(cfg, st, false); err != nil {
			logger.Error("Validation failed: %v", err)
			return err
		}
		logger.Info("Validation successful")
		return nil
	}

	// Ensure export directory exists
	if err := cfg.EnsureDirs(); err != nil {
		logger.Error("Failed to create directories: %v", err)
		return err
	}

	// Connect to database
	logger.Info("Connecting to database: %s@%s:%d/%s",
		cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBService)

	database, err := connectDatabase(ctx, cfg)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		return err
	}
	defer database.Close()

	logger.Info("Database connection established")

	// Execute export
	result, err := executeExport(ctx, cfg, database, st, logger)
	if err != nil {
		logger.Error("Export failed: %v", err)
		return err
	}

	// Print summary
	printSummary(result, cfg, logger)

	// Exit with appropriate code
	if result.FailedCount > 0 {
		logger.Info("Export completed with %d failures", result.FailedCount)
		os.Exit(2)
	}

	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	cfg := config.FromCommand(cmd)

	logger := logging.New(cfg.Verbose)
	defer logger.Close()

	logger.Info("Validating ora2csv configuration")

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
		return err
	}

	// Load state file (no S3 for validation)
	st, err := state.Load(cfg.StateFile, nil, "")
	if err != nil {
		logger.Error("Failed to load state file: %v", err)
		return fmt.Errorf("failed to load state file: %w", err)
	}

	// Get test connection flag
	testConn, _ := cmd.Flags().GetBool("test-connection")

	// Run validation
	if err := exporter.Validate(cfg, st, testConn); err != nil {
		logger.Error("Validation failed: %v", err)
		return err
	}

	logger.Info("Configuration validation: OK")
	logger.Info("State file: OK (%d entities, %d active)", st.TotalCount(), st.ActiveCount())
	logger.Info("SQL files: OK")

	if testConn {
		logger.Info("Database connection: OK")
	}

	return nil
}
