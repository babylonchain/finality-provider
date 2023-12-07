package config

import "fmt"

const (
	DefaultBackend = "bbolt"
	DefaultDBName  = "default"
)

type DatabaseConfig struct {
	Backend string `long:"backend" description:"Possible database to choose as backend"`
	Name    string `long:"name" description:"The name of the database"`
}

func NewDatabaseConfig(backend string, name string) (*DatabaseConfig, error) {
	// TODO: add more supported DB types, currently we only support bbolt
	if backend != DefaultBackend {
		return nil, fmt.Errorf("unsupported DB backend")
	}

	if name == "" {
		return nil, fmt.Errorf("bucket name should not be empty")
	}

	return &DatabaseConfig{
		Backend: backend,
		Name:    name,
	}, nil
}

func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Backend: DefaultBackend,
		Name:    DefaultDBName,
	}
}
