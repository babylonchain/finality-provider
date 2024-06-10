package config

import (
	"time"

	"github.com/babylonchain/finality-provider/wasmdclient/config"
)

type WasmdConfig struct {
	Key            string        `long:"key" description:"name of the key to sign transactions with"`
	ChainID        string        `long:"chain-id" description:"chain id of the chain to connect to"`
	RPCAddr        string        `long:"rpc-address" description:"address of the rpc server to connect to"`
	GRPCAddr       string        `long:"grpc-address" description:"address of the grpc server to connect to"`
	AccountPrefix  string        `long:"acc-prefix" description:"account prefix to use for addresses"`
	KeyringBackend string        `long:"keyring-type" description:"type of keyring to use"`
	GasAdjustment  float64       `long:"gas-adjustment" description:"adjustment factor when using gas estimation"`
	GasPrices      string        `long:"gas-prices" description:"comma separated minimum gas prices to accept for transactions"`
	KeyDirectory   string        `long:"key-dir" description:"directory to store keys in"`
	Debug          bool          `long:"debug" description:"flag to print debug output"`
	Timeout        time.Duration `long:"timeout" description:"client timeout when doing queries"`
	BlockTimeout   time.Duration `long:"block-timeout" description:"block timeout when waiting for block events"`
	OutputFormat   string        `long:"output-format" description:"default output when printint responses"`
	SignModeStr    string        `long:"sign-mode" description:"sign mode to use"`
}

func DefaultWasmdConfig() *WasmdConfig {
	// fill up the config from dc config
	return &WasmdConfig{
		Key:            "validator",
		ChainID:        "wasmd-test",
		RPCAddr:        "http://localhost:2990",
		GRPCAddr:       "https://localhost:9090",
		AccountPrefix:  "wasm",
		KeyringBackend: "test",
		GasAdjustment:  1.3,
		GasPrices:      "1ustake",
		Debug:          true,
		Timeout:        20 * time.Second,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: "direct",
		SignModeStr:  "",
	}
}

func WasmdConfigToQueryClientConfig(wc *WasmdConfig) *config.CosmosChainConfig {
	return &config.CosmosChainConfig{
		Key:              wc.Key,
		ChainID:          wc.ChainID,
		RPCAddr:          wc.RPCAddr,
		AccountPrefix:    wc.AccountPrefix,
		KeyringBackend:   wc.KeyringBackend,
		GasAdjustment:    wc.GasAdjustment,
		GasPrices:        wc.GasPrices,
		KeyDirectory:     wc.KeyDirectory,
		Debug:            wc.Debug,
		Timeout:          wc.Timeout,
		BlockTimeout:     wc.BlockTimeout,
		OutputFormat:     wc.OutputFormat,
		SignModeStr:      wc.SignModeStr,
		SubmitterAddress: "",
	}
}
