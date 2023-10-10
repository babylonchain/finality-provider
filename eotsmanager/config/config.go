package config

import "fmt"

const (
	TypeLocal = "local"

	DefaultDBBackend      = "bbolt"
	DefaultDBPath         = "bbolt-eots.db"
	DefaultDBName         = "eots-default"
	DefaultKeyringBackend = "test"
	DefaultMode           = TypeLocal
)

type Config struct {
	Mode           string `long:"mode" description:"Indicates in which mode the EOTS manager is running"`
	DBBackend      string `long:"dbbackend" description:"Possible database to choose as backend"`
	DBPath         string `long:"dbpath" description:"The path that stores the database file"`
	DBName         string `long:"dbname" description:"The name of the database"`
	KeyDirectory   string `long:"key-dir" description:"Directory to store keys in"`
	KeyringBackend string `long:"keyring-type" description:"Type of keyring to use"`
}

func NewConfig(mode, backend, path, name, keyDir, keyringBackend string) (*Config, error) {
	if backend != DefaultDBBackend {
		return nil, fmt.Errorf("unsupported DB backend")
	}

	if path == "" {
		return nil, fmt.Errorf("DB path should not be empty")
	}

	if name == "" {
		return nil, fmt.Errorf("bucket name should not be empty")
	}

	return &Config{
		Mode:           mode,
		KeyDirectory:   keyDir,
		DBBackend:      backend,
		DBPath:         path,
		DBName:         name,
		KeyringBackend: keyringBackend,
	}, nil
}

func DefaultEOTSManagerConfig() Config {
	return Config{
		DBBackend:      DefaultDBBackend,
		DBPath:         DefaultDBPath,
		DBName:         DefaultDBName,
		KeyringBackend: DefaultKeyringBackend,
		Mode:           DefaultMode,
	}
}
