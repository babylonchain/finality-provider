package valcfg

import (
	"fmt"
)

const (
	DefaultEOTSManagerDBBackend = "bbolt"
	DefaultEOTSManagerDBPath    = "bbolt-eots.db"
	DefaultEOTSManagerDBName    = "eots-default"
	DefaultEOTSManagerMode      = "local"
)

type EOTSManagerConfig struct {
	Mode      string `long:"mode" description:"Indicates in which mode the EOTS manager is running" choice:"local" choice:"remote"`
	DBBackend string `long:"dbbackend" description:"Possible database to choose as backend"`
	DBPath    string `long:"dbpath" description:"The path that stores the database file"`
	DBName    string `long:"dbname" description:"The name of the database"`
}

func NewEOTSManagerConfig(backend string, path string, name string) (*EOTSManagerConfig, error) {
	if backend != DefaultEOTSManagerDBBackend {
		return nil, fmt.Errorf("unsupported DB backend")
	}

	if path == "" {
		return nil, fmt.Errorf("DB path should not be empty")
	}

	if name == "" {
		return nil, fmt.Errorf("bucket name should not be empty")
	}

	return &EOTSManagerConfig{
		DBBackend: backend,
		DBPath:    path,
		DBName:    name,
	}, nil
}

func DefaultEOTSManagerConfig() EOTSManagerConfig {
	return EOTSManagerConfig{
		DBBackend: DefaultEOTSManagerDBBackend,
		DBPath:    DefaultEOTSManagerDBPath,
		DBName:    DefaultEOTSManagerDBName,
		Mode:      DefaultEOTSManagerMode,
	}
}
