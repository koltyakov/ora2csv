package config

const (
	// Default values
	DefaultDBHost             = "dbserver"
	DefaultDBPort             = 1521
	DefaultDBService          = "ORCL"
	DefaultDBUser             = "system"
	DefaultStateFile          = "./state.json"
	DefaultSQLDir             = "./sql"
	DefaultExportDir          = "./export"
	DefaultDaysBack           = 30
	DefaultConnectTimeoutSecs = 30
	DefaultQueryTimeoutSecs   = 300 // 5 minutes

	// S3 defaults
	DefaultS3PartSize = 5 * 1024 * 1024 // 5MB
)

const (
	// Environment variable names
	EnvDBPassword = "ORA2CSV_DB_PASSWORD"
	EnvPrefix     = "ORA2CSV"

	// S3 environment variable names (ora2csv-specific)
	// Note: AWS credentials and region use standard AWS env vars (AWS_ACCESS_KEY_ID,
	// AWS_REGION, etc.) which are automatically picked up by the AWS SDK. This enables
	// compatibility with aws-vault, aws-cli, and other AWS tools.
	EnvS3Bucket   = "ORA2CSV_S3_BUCKET"
	EnvS3Prefix   = "ORA2CSV_S3_PREFIX"
	EnvS3Endpoint = "ORA2CSV_S3_ENDPOINT"
)
