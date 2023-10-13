package valcfg

import (
	eotscfg "github.com/babylonchain/btc-validator/eotsmanager/config"
)

const (
	DefaultEOTSManagerDBBackend = "bbolt"
	DefaultEOTSManagerDBPath    = "bbolt-eots.db"
	DefaultEOTSManagerDBName    = "eots-default"
)

type EOTSManagerConfig struct {
	DBBackend string `long:"dbbackend" description:"Possible database to choose as backend"`
	DBPath    string `long:"dbpath" description:"The path that stores the database file"`
	DBName    string `long:"dbname" description:"The name of the database"`
}

func AppConfigToEOTSManagerConfig(appCfg *Config) (*eotscfg.Config, error) {
	dbCfg, err := eotscfg.NewDatabaseConfig(
		appCfg.EOTSManagerConfig.DBBackend,
		appCfg.EOTSManagerConfig.DBPath,
		appCfg.EOTSManagerConfig.DBName,
	)
	if err != nil {
		return nil, err
	}
	return &eotscfg.Config{
		LogLevel:       appCfg.DebugLevel,
		EOTSDir:        appCfg.ValdDir,
		ConfigFile:     appCfg.ConfigFile,
		KeyDirectory:   appCfg.BabylonConfig.KeyDirectory,
		KeyringBackend: appCfg.BabylonConfig.KeyringBackend,
		DatabaseConfig: dbCfg,
	}, nil
}

func DefaultEOTSManagerConfig() EOTSManagerConfig {
	return EOTSManagerConfig{
		DBBackend: DefaultEOTSManagerDBBackend,
		DBPath:    DefaultEOTSManagerDBPath,
		DBName:    DefaultEOTSManagerDBName,
	}
}
