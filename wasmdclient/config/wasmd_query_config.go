package config

import (
	"fmt"
	"net/url"
	"time"
)

// WasmdConfig defines configuration for the Babylon query client
type WasmdQueryConfig struct {
	RPCAddr string        `mapstructure:"rpc-addr"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func (cfg *WasmdQueryConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("cfg.RPCAddr is not correctly formatted: %w", err)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("cfg.Timeout must be positive")
	}
	return nil
}

func DefaultWasmdQueryConfig() WasmdQueryConfig {
	return WasmdQueryConfig{
		RPCAddr: "http://localhost:26657",
		Timeout: 20 * time.Second,
	}
}
