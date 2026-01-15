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
)

const (
	// Environment variable names
	EnvDBPassword = "ORASYSTEMPASS"
	EnvPrefix     = "ORA2CSV"
)
