package valcfg

import "fmt"

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

func NewDatabaseConfig(dbType string, path string, name string) (*DatabaseConfig, error) {
	// TODO: add more supported DB types, currently we only support bbolt
	if dbType != DefaultDBType {
		return nil, fmt.Errorf("unsupported DB type")
	}

	if path == "" {
		return nil, fmt.Errorf("DB path should not be empty")
	}

	if name == "" {
		return nil, fmt.Errorf("bucket name should not be empty")
	}

	return &DatabaseConfig{
		DbType: dbType,
		Path:   path,
		Name:   name,
	}, nil
}

func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		DbType: DefaultDBType,
		Path:   DefaultDBPath,
		Name:   DefaultDBName,
	}
}
