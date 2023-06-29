package valcfg

// ChainConfig houses the configuration options that govern which chain/network
// we operate on
type ChainConfig struct {
	Network string
}

func DefaultChainConfig() *ChainConfig {
	return &ChainConfig{
		Network: "testnet3",
	}
}

// Config is the main config for the tapd cli command
type Config struct {
	ChainConfig *ChainConfig
}

func DefaultConfig() Config {
	return Config{
		ChainConfig: DefaultChainConfig(),
	}
}
