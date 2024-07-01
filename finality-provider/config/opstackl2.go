package config

import (
	"fmt"
	"net/url"
	"time"

	cwcfg "github.com/babylonchain/finality-provider/cosmwasmclient/config"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

type OPStackL2Config struct {
	OPStackL2RPCAddress     string `long:"opstackl2-rpc-address" description:"the rpc address of the op-stack-l2 node to connect to"`
	OPFinalityGadgetAddress string `long:"op-finality-gadget"    description:"the contract address of the op-finality-gadget"`
	// Below configurations are needed for the Babylon client
	Key            string        `long:"key"                   description:"name of the key to sign transactions with"`
	ChainID        string        `long:"chain-id"              description:"chain id of the chain to connect to"`
	RPCAddr        string        `long:"rpc-address"           description:"address of the rpc server to connect to"`
	GRPCAddr       string        `long:"grpc-address"          description:"address of the grpc server to connect to"`
	AccountPrefix  string        `long:"acc-prefix"            description:"account prefix to use for addresses"`
	KeyringBackend string        `long:"keyring-type"          description:"type of keyring to use"`
	GasAdjustment  float64       `long:"gas-adjustment"        description:"adjustment factor when using gas estimation"`
	GasPrices      string        `long:"gas-prices"            description:"comma separated minimum gas prices to accept for transactions"`
	KeyDirectory   string        `long:"key-dir"               description:"directory to store keys in"`
	Debug          bool          `long:"debug"                 description:"flag to print debug output"`
	Timeout        time.Duration `long:"timeout"               description:"client timeout when doing queries"`
	BlockTimeout   time.Duration `long:"block-timeout"         description:"block timeout when waiting for block events"`
	OutputFormat   string        `long:"output-format"         description:"default output when printint responses"`
	SignModeStr    string        `long:"sign-mode"             description:"sign mode to use"`
}

func (cfg *OPStackL2Config) Validate() error {
	if cfg.OPStackL2RPCAddress == "" {
		return fmt.Errorf("opstackl2-rpc-address is required")
	}
	if cfg.OPFinalityGadgetAddress == "" {
		return fmt.Errorf("the contract address of the op-finality-gadget is required")
	}
	_, err := sdktypes.AccAddressFromBech32(cfg.OPFinalityGadgetAddress)
	if err != nil {
		return fmt.Errorf("op-finality-gadget: invalid bech32 address: %w", err)
	}
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("rpc-addr is not correctly formatted: %w", err)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if cfg.BlockTimeout < 0 {
		return fmt.Errorf("block-timeout can't be negative")
	}
	return nil
}

func (cfg *OPStackL2Config) ToCosmwasmConfig() cwcfg.CosmwasmConfig {
	return cwcfg.CosmwasmConfig{
		Key:              cfg.Key,
		ChainID:          cfg.ChainID,
		RPCAddr:          cfg.RPCAddr,
		AccountPrefix:    cfg.AccountPrefix,
		KeyringBackend:   cfg.KeyringBackend,
		GasAdjustment:    cfg.GasAdjustment,
		GasPrices:        cfg.GasPrices,
		KeyDirectory:     cfg.KeyDirectory,
		Debug:            cfg.Debug,
		Timeout:          cfg.Timeout,
		BlockTimeout:     cfg.BlockTimeout,
		OutputFormat:     cfg.OutputFormat,
		SignModeStr:      cfg.SignModeStr,
		SubmitterAddress: "",
	}
}
