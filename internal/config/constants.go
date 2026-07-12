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
	DefaultWatermarkLagSecs   = 0
	DefaultNullValue          = ""

	// S3 defaults
	DefaultS3PartSize          int64 = 5 * 1024 * 1024
	MaxS3PartSize              int64 = 5 * 1024 * 1024 * 1024
	DefaultS3Concurrency             = 5
	DefaultS3UploadTimeoutSecs       = 300
)

const (
	// Environment variable names
	EnvDBPassword = "ORA2CSV_DB_PASSWORD"
	EnvPrefix     = "ORA2CSV"

	// S3 environment variable names (ora2csv-specific)
	// Note: AWS credentials and region use standard AWS env vars (AWS_ACCESS_KEY_ID,
	// AWS_REGION, etc.) which are automatically picked up by the AWS SDK. This enables
	// compatibility with aws-vault, aws-cli, and other AWS tools.
	EnvS3Bucket        = "ORA2CSV_S3_BUCKET"
	EnvS3Prefix        = "ORA2CSV_S3_PREFIX"
	EnvS3Endpoint      = "ORA2CSV_S3_ENDPOINT"
	EnvWatermarkLag    = "ORA2CSV_WATERMARK_LAG"
	EnvNullValue       = "ORA2CSV_NULL_VALUE"
	EnvS3UploadTimeout = "ORA2CSV_S3_UPLOAD_TIMEOUT"
	EnvS3PartSize      = "ORA2CSV_S3_PART_SIZE"
	EnvS3Concurrency   = "ORA2CSV_S3_CONCURRENCY"
)
