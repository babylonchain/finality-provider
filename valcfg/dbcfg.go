package valcfg

const (
	DefaultDBType = "bbolt"
	DefaultDBPath = "bbolt.db"
	DefaultDBName = "default"
)

type DatabaseConfig struct {
	DbType string
	Path   string
	Name   string
}

func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		DbType: DefaultDBType,
		Path:   DefaultDBPath,
		Name:   DefaultDBName,
	}
}
