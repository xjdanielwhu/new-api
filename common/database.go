package common

type DatabaseType = string

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
	DatabaseTypeClickHouse = "clickhouse"
)

// 上游把数据库类型收敛成 MainDatabaseType/LogDatabaseType 访问器，v2 保留原有的
// UsingSQLite/UsingMySQL/UsingPostgreSQL/LogSqlType 变量作为存储，避免改动散落在
// 各处的既有引用（含直接赋值这些变量的测试）。读写两侧都走同一份状态。
var UsingSQLite = false
var UsingPostgreSQL = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

func MainDatabaseType() DatabaseType {
	switch {
	case UsingMySQL:
		return DatabaseTypeMySQL
	case UsingPostgreSQL:
		return DatabaseTypePostgreSQL
	default:
		return DatabaseTypeSQLite
	}
}

func LogDatabaseType() DatabaseType {
	return LogSqlType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	UsingMySQL = databaseType == DatabaseTypeMySQL
	UsingPostgreSQL = databaseType == DatabaseTypePostgreSQL
	UsingSQLite = databaseType == DatabaseTypeSQLite
}

func SetLogDatabaseType(databaseType DatabaseType) {
	LogSqlType = databaseType
	UsingClickHouse = databaseType == DatabaseTypeClickHouse
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	SetMainDatabaseType(mainType)
	SetLogDatabaseType(logType)
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return MainDatabaseType() == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return LogDatabaseType() == databaseType
}

var SQLitePath = "one-api.db?_busy_timeout=30000"
