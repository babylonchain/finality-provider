package config

import (
	"fmt"
	"net/url"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
)

// CosmwasmConfig defines configuration for the Babylon client
// adapted from https://github.com/strangelove-ventures/lens/blob/v0.5.1/client/config.go
type CosmwasmConfig struct {
	Key                       string        `mapstructure:"key"`
	ChainID                   string        `mapstructure:"chain-id"`
	RPCAddr                   string        `mapstructure:"rpc-addr"`
	GRPCAddr                  string        `mapstructure:"grpc-addr"`
	AccountPrefix             string        `mapstructure:"account-prefix"`
	KeyringBackend            string        `mapstructure:"keyring-backend"`
	GasAdjustment             float64       `mapstructure:"gas-adjustment"`
	GasPrices                 string        `mapstructure:"gas-prices"`
	KeyDirectory              string        `mapstructure:"key-directory"`
	Debug                     bool          `mapstructure:"debug"`
	Timeout                   time.Duration `mapstructure:"timeout"`
	BlockTimeout              time.Duration `mapstructure:"block-timeout"`
	OutputFormat              string        `mapstructure:"output-format"`
	SignModeStr               string        `mapstructure:"sign-mode"`
	SubmitterAddress          string        `mapstructure:"submitter-address"`
	BabylonContractAddress    string        `mapstructure:"babylon-contract-address"`
	BtcStakingContractAddress string        `mapstructure:"btc-staking-contract-address"`
}

func (cfg *CosmwasmConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("rpc-addr is not correctly formatted: %w", err)
	}

	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if cfg.BlockTimeout < 0 {
		return fmt.Errorf("block-timeout can't be negative")
	}

	if cfg.BabylonContractAddress == "" {
		return fmt.Errorf("babylon-contract-address is required")
	}
	_, err := sdk.AccAddressFromBech32(cfg.BabylonContractAddress)
	if err != nil {
		return fmt.Errorf("babylon-contract-address: invalid bech32 address: %w", err)
	}

	if cfg.BtcStakingContractAddress == "" {
		return fmt.Errorf("btc-staking-contract-address is required")
	}
	_, err = sdk.AccAddressFromBech32(cfg.BtcStakingContractAddress)
	if err != nil {
		return fmt.Errorf("babylon-contract-address: invalid bech32 address: %w", err)
	}

	return nil
}

func (cfg *CosmwasmConfig) ToCosmosProviderConfig() cosmos.CosmosProviderConfig {
	return cosmos.CosmosProviderConfig{
		Key:            cfg.Key,
		ChainID:        cfg.ChainID,
		RPCAddr:        cfg.RPCAddr,
		AccountPrefix:  cfg.AccountPrefix,
		KeyringBackend: cfg.KeyringBackend,
		GasAdjustment:  cfg.GasAdjustment,
		GasPrices:      cfg.GasPrices,
		KeyDirectory:   cfg.KeyDirectory,
		Debug:          cfg.Debug,
		Timeout:        cfg.Timeout.String(),
		BlockTimeout:   cfg.BlockTimeout.String(),
		OutputFormat:   cfg.OutputFormat,
		SignModeStr:    cfg.SignModeStr,
	}
}
