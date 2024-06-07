package config

import (
	"fmt"
	"net/url"
	"time"
)

// CosmosChainConfig defines configuration for the Babylon query client
type CosmosChainQueryConfig struct {
	RPCAddr string        `mapstructure:"rpc-addr"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func (cfg *CosmosChainQueryConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("cfg.RPCAddr is not correctly formatted: %w", err)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("cfg.Timeout must be positive")
	}
	return nil
}

func DefaultBabylonQueryConfig() CosmosChainQueryConfig {
	return CosmosChainQueryConfig{
		RPCAddr: "http://localhost:26657",
		Timeout: 20 * time.Second,
	}
}
