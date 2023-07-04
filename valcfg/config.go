package valcfg

// Config is the main config for the tapd cli command
type Config struct {
	*DatabaseConfig
}

func DefaultConfig() Config {
	return Config{
		DatabaseConfig: DefaultDatabaseConfig(),
	}
}
