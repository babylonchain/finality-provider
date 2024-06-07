package config

import (
	"time"

	bbncfg "github.com/babylonchain/babylon/client/config"
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

func DefaultWasmdConfig() WasmdConfig {
	dc := bbncfg.DefaultBabylonConfig()
	// fill up the config from dc config
	return WasmdConfig{
		Key:            dc.Key,
		ChainID:        dc.ChainID,
		RPCAddr:        dc.RPCAddr,
		GRPCAddr:       dc.GRPCAddr,
		AccountPrefix:  dc.AccountPrefix,
		KeyringBackend: dc.KeyringBackend,
		GasAdjustment:  1.5,
		GasPrices:      "0.002ubbn",
		Debug:          dc.Debug,
		Timeout:        dc.Timeout,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: dc.OutputFormat,
		SignModeStr:  dc.SignModeStr,
	}
}

func DefaultWasmdConfig() BabylonConfig {
	return BabylonConfig{
		Key:     "node0",
		ChainID: "chain-test",
		// see https://docs.cosmos.network/master/core/grpc_rest.html for default ports
		// TODO: configure HTTPS for Babylon's RPC server
		// TODO: how to use Cosmos SDK's RPC server (port 1317) rather than Tendermint's RPC server (port 26657)?
		RPCAddr: "http://localhost:26657",
		// TODO: how to support GRPC in the Babylon client?
		GRPCAddr:         "https://localhost:9090",
		AccountPrefix:    "bbn",
		KeyringBackend:   "test",
		GasAdjustment:    1.2,
		GasPrices:        "0.01ubbn",
		KeyDirectory:     defaultBabylonHome(),
		Debug:            true,
		Timeout:          20 * time.Second,
		OutputFormat:     "json",
		SignModeStr:      "direct",
		SubmitterAddress: "bbn1v6k7k9s8md3k29cu9runasstq5zaa0lpznk27w", // this is currently a placeholder, will not recognized by Babylon
	}
}

func WasmdConfigToQueryClientConfig(wc *WasmdConfig) bbncfg.BabylonConfig {
	return bbncfg.BabylonConfig{
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
