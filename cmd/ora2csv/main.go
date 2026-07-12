package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
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

var errExportFailures = errors.New("one or more entities failed")

var rootCmd = newRootCommand(resolvedVersion(), buildTime)

func newRootCommand(commandVersion, commandBuildTime string) *cobra.Command {
	root := &cobra.Command{
		Use:           "ora2csv",
		Short:         "Oracle to CSV exporter with state management",
		Long:          "ora2csv exports data from Oracle to CSV files with incremental sync.\nIt streams data directly from Oracle to CSV without storing entire exports in memory.",
		Version:       fmt.Sprintf("%s (built: %s)", commandVersion, commandBuildTime),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	export := &cobra.Command{
		Use:   "export",
		Short: "Export data from Oracle to CSV",
		Long:  "Export data from Oracle database to CSV files based on state.json configuration",
		Args:  cobra.NoArgs,
		RunE:  runExport,
	}
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and SQL files",
		Long:  "Validate configuration, check SQL files exist, and optionally test database connection",
		Args:  cobra.NoArgs,
		RunE:  runValidate,
	}

	// Common flags
	root.PersistentFlags().String("db-host", config.DefaultDBHost, "Database host")
	root.PersistentFlags().Int("db-port", config.DefaultDBPort, "Database port")
	root.PersistentFlags().String("db-service", config.DefaultDBService, "Database service name")
	root.PersistentFlags().String("db-user", config.DefaultDBUser, "Database user")
	root.PersistentFlags().String("state-file", config.DefaultStateFile, "Path to state.json file")
	root.PersistentFlags().String("sql-dir", config.DefaultSQLDir, "Path to SQL directory")
	root.PersistentFlags().String("export-dir", config.DefaultExportDir, "Path to export directory")
	root.PersistentFlags().Int("days-back", config.DefaultDaysBack, "Default days to look back for first run")
	root.PersistentFlags().Bool("dry-run", false, "Validate without executing")
	root.PersistentFlags().Bool("verbose", false, "Enable verbose logging")
	root.PersistentFlags().Duration("connect-timeout", config.DefaultConnectTimeoutSecs*time.Second, "Connection timeout")
	root.PersistentFlags().Duration("query-timeout", config.DefaultQueryTimeoutSecs*time.Second, "Query timeout")
	root.PersistentFlags().Duration("watermark-lag", config.DefaultWatermarkLagSecs*time.Second, "Delay the Oracle watermark to allow recent transactions to commit")
	root.PersistentFlags().String("null-value", config.DefaultNullValue, "CSV value used for SQL NULL")

	// S3 flags
	root.PersistentFlags().String("s3-bucket", "", "S3 bucket name")
	root.PersistentFlags().String("s3-prefix", "", "S3 key prefix")
	root.PersistentFlags().String("s3-access-key", "", "S3 access key (for S3-compatible services)")
	root.PersistentFlags().String("s3-secret-key", "", "S3 secret key (for S3-compatible services)")
	root.PersistentFlags().String("s3-session-token", "", "S3 session token (for S3-compatible services)")
	root.PersistentFlags().String("s3-endpoint", "", "S3 endpoint URL (for S3-compatible services like MinIO)")
	root.PersistentFlags().Duration("s3-upload-timeout", config.DefaultS3UploadTimeoutSecs*time.Second, "S3 upload timeout")
	root.PersistentFlags().Int64("s3-part-size", config.DefaultS3PartSize, "S3 multipart part size in bytes")
	root.PersistentFlags().Int("s3-concurrency", config.DefaultS3Concurrency, "S3 multipart upload concurrency")
	root.PersistentFlags().Bool("s3-allow-insecure-endpoint", false, "Allow a plaintext HTTP S3-compatible endpoint")

	// Validate-specific flags
	validate.Flags().Bool("test-connection", false, "Test database connection")
	root.AddCommand(export, validate)
	return root
}

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if errors.Is(err, errExportFailures) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// setupContext creates a context with cancellation and signal handling
func setupContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
func executeExport(ctx context.Context, cfg *config.Config, database *db.OracleDB, st *state.File, logger *logging.Logger, s3Client *storage.S3Client) (*types.ExportResult, error) {
	// Create and run exporter
	exp := exporter.New(cfg, database, st, logger, s3Client)
	return exp.Run(ctx)
}

// printSummary prints the export result summary
func printSummary(result *types.ExportResult, cfg *config.Config, logger *logging.Logger, runErr error) {
	duration := result.Duration
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60

	logger.Info("==================================================")
	interrupted := errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded)
	if interrupted {
		logger.Error("Export interrupted")
	} else if runErr != nil {
		logger.Error("Export stopped with an error")
	} else if result.FailedCount > 0 {
		logger.Error("Export completed with failures")
	} else {
		logger.Info("Export completed successfully")
	}
	logger.Info("Total duration: %dm %ds", minutes, seconds)
	logger.Info("Total entities: %d", result.TotalEntities)
	logger.Info("Successfully processed: %d", result.SuccessCount)
	if result.FailedCount > 0 {
		logger.Error("Failed entities: %d", result.FailedCount)
	}
	if runErr != nil {
		logger.Info("Not processed: %d", result.SkippedCount)
	} else {
		logger.Info("Skipped (inactive): %d", result.SkippedCount)
	}
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
	cfg, err := config.FromCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup context with signal handling
	ctx, cancel := setupContext()
	defer cancel()

	// Create logger
	logger := logging.New(cfg.Verbose)
	defer func() {
		if closeErr := logger.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close logger: %v\n", closeErr)
		}
	}()

	logger.Info("Starting ora2csv v%s (built: %s)", version, buildTime)

	// Validate configuration (including S3)
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
		return err
	}

	// Dry run is deliberately local and side-effect free.
	if cfg.DryRun {
		logger.Info("Dry run mode - validating local configuration only")
		st, err := state.Load(cfg.StateFile, nil, "")
		if err != nil {
			logger.Error("Failed to load state file: %v", err)
			return fmt.Errorf("failed to load state file: %w", err)
		}
		if err := exporter.Validate(cfg, st, false); err != nil {
			logger.Error("Validation failed: %v", err)
			return err
		}
		logger.Info("Validation successful")
		return nil
	}

	stateLock, err := acquireStateLock(cfg.StateFile)
	if err != nil {
		logger.Error("Failed to acquire state lock: %v", err)
		return err
	}
	defer func() {
		if closeErr := stateLock.Close(); closeErr != nil {
			logger.Error("Failed to release state lock: %v", closeErr)
		}
	}()

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
		defer checkCancel()
		if err := s3Client.CheckConnection(checkCtx); err != nil {
			logger.Error("S3 connectivity check failed: %v", err)
			return fmt.Errorf("S3 connectivity check failed: %w", err)
		}
		logger.Info("S3 connectivity verified")
	}

	// Load state file (with S3 sync if enabled)
	st, err := state.LoadContext(ctx, cfg.StateFile, s3Client, s3StateKey)
	if err != nil {
		logger.Error("Failed to load state file: %v", err)
		return fmt.Errorf("failed to load state file: %w", err)
	}

	logger.Info("Loaded state file: %s (%d entities, %d active)",
		cfg.StateFile, st.TotalCount(), st.ActiveCount())

	if err := exporter.Validate(cfg, st, false); err != nil {
		logger.Error("Validation failed: %v", err)
		return err
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
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Error("Failed to close database connection: %v", closeErr)
		}
	}()

	logger.Info("Database connection established")

	// Execute export
	result, err := executeExport(ctx, cfg, database, st, logger, s3Client)
	if result != nil {
		printSummary(result, cfg, logger, err)
	}
	if err != nil {
		return err
	}

	// Exit with appropriate code
	if result.FailedCount > 0 {
		logger.Info("Export completed with %d failures", result.FailedCount)
		return errExportFailures
	}

	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger := logging.New(cfg.Verbose)
	defer func() {
		if closeErr := logger.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close logger: %v\n", closeErr)
		}
	}()

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
