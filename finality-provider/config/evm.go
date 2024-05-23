package config

import (
	"fmt"
	"net/url"
)

const (
	defaultEVMRPCAddr = "http://127.0.0.1:8545"
)

type EVMConfig struct {
	RPCAddr string `long:"rpc-address" description:"address of the rpc server to connect to"`
}

func DefaultEVMConfig() EVMConfig {
	return EVMConfig{
		RPCAddr: defaultEVMRPCAddr,
	}
}

func (cfg *EVMConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("rpc-addr is not correctly formatted: %w", err)
	}
	return nil
}
