package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/koltyakov/ora2csv/internal/config"
	"github.com/koltyakov/ora2csv/internal/db"
	"github.com/koltyakov/ora2csv/internal/exporter"
	"github.com/koltyakov/ora2csv/internal/logging"
	"github.com/koltyakov/ora2csv/internal/state"
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
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and SQL files",
	Long:  "Validate configuration, check SQL files exist, and optionally test database connection",
	RunE:  runValidate,
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

	// Validate-specific flags
	validateCmd.Flags().Bool("test-connection", false, "Test database connection")
}

// loadConfig loads configuration from flags and environment variables
func loadConfig(cmd *cobra.Command) *config.Config {
	v := viper.New()

	// Bind flags to viper
	flags := []struct {
		name string
		key  string
	}{
		{"db-host", "db_host"},
		{"db-port", "db_port"},
		{"db-service", "db_service"},
		{"db-user", "db_user"},
		{"state-file", "state_file"},
		{"sql-dir", "sql_dir"},
		{"export-dir", "export_dir"},
		{"days-back", "days_back"},
		{"dry-run", "dry_run"},
		{"verbose", "verbose"},
		{"connect-timeout", "connect_timeout"},
		{"query-timeout", "query_timeout"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.name)
		if flag != nil {
			_ = v.BindPFlag(f.key, flag)
		}
	}

	// Enable environment variable reading
	v.SetEnvPrefix(config.EnvPrefix)
	v.AutomaticEnv()
	v.BindEnv("db_password", config.EnvDBPassword)

	// Set defaults from config package
	v.SetDefault("db_host", config.DefaultDBHost)
	v.SetDefault("db_port", config.DefaultDBPort)
	v.SetDefault("db_service", config.DefaultDBService)
	v.SetDefault("db_user", config.DefaultDBUser)
	v.SetDefault("state_file", config.DefaultStateFile)
	v.SetDefault("sql_dir", config.DefaultSQLDir)
	v.SetDefault("export_dir", config.DefaultExportDir)
	v.SetDefault("days_back", config.DefaultDaysBack)
	v.SetDefault("dry_run", false)
	v.SetDefault("verbose", false)
	v.SetDefault("connect_timeout", config.DefaultConnectTimeoutSecs*time.Second)
	v.SetDefault("query_timeout", config.DefaultQueryTimeoutSecs*time.Second)

	// Unmarshal to config
	result := &config.Config{}
	if err := v.Unmarshal(result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal config: %v\n", err)
		os.Exit(1)
	}

	// Set durations from duration flags
	result.ConnectTimeout = v.GetDuration("connect_timeout")
	result.QueryTimeout = v.GetDuration("query_timeout")

	return result
}

func main() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(validateCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runExport(cmd *cobra.Command, args []string) error {
	// Load configuration from flags and environment
	cfg := loadConfig(cmd)

	// Create context for this run
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for this command
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Create logger
	logger := logging.New(cfg.Verbose)
	defer logger.Close()

	logger.Info("Starting ora2csv v%s (built: %s)", version, buildTime)

	// Load state file
	st, err := state.Load(cfg.StateFile)
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

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
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
		logger.Error("Failed to connect to database: %v", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	logger.Info("Database connection established")

	// Create exporter with query timeout
	queryCtx, queryCancel := context.WithTimeout(ctx, cfg.QueryTimeout)
	defer queryCancel()

	exp := exporter.New(cfg, database, st, logger)

	// Run export
	result, err := exp.Run(queryCtx)
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
	cfg := loadConfig(cmd)

	logger := logging.New(cfg.Verbose)
	defer logger.Close()

	logger.Info("Validating ora2csv configuration")

	// Load state file
	st, err := state.Load(cfg.StateFile)
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
