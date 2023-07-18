package valcfg

import "fmt"

const (
	DefaultBackend = "bbolt"
	DefaultDBPath  = "bbolt.db"
	DefaultDBName  = "default"
)

type DatabaseConfig struct {
	Backend string `long:"backend" description:"Possible database to choose as backend"`
	Path    string `long:"path" description:"The path that stores the database file"`
	Name    string `long:"path" description:"The name of the database"`
}

func NewDatabaseConfig(backend string, path string, name string) (*DatabaseConfig, error) {
	// TODO: add more supported DB types, currently we only support bbolt
	if backend != DefaultBackend {
		return nil, fmt.Errorf("unsupported DB backend")
	}

	if path == "" {
		return nil, fmt.Errorf("DB path should not be empty")
	}

	if name == "" {
		return nil, fmt.Errorf("bucket name should not be empty")
	}

	return &DatabaseConfig{
		Backend: backend,
		Path:    path,
		Name:    name,
	}, nil
}

func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Backend: DefaultBackend,
		Path:    DefaultDBPath,
		Name:    DefaultDBName,
	}
}
