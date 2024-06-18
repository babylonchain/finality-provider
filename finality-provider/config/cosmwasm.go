package config

import (
	"time"

	"github.com/babylonchain/finality-provider/cosmwasmclient/config"
)

type CosmwasmConfig struct {
	Key                       string        `long:"key" description:"name of the key to sign transactions with"`
	ChainID                   string        `long:"chain-id" description:"chain id of the chain to connect to"`
	RPCAddr                   string        `long:"rpc-address" description:"address of the rpc server to connect to"`
	GRPCAddr                  string        `long:"grpc-address" description:"address of the grpc server to connect to"`
	AccountPrefix             string        `long:"acc-prefix" description:"account prefix to use for addresses"`
	KeyringBackend            string        `long:"keyring-type" description:"type of keyring to use"`
	GasAdjustment             float64       `long:"gas-adjustment" description:"adjustment factor when using gas estimation"`
	GasPrices                 string        `long:"gas-prices" description:"comma separated minimum gas prices to accept for transactions"`
	KeyDirectory              string        `long:"key-dir" description:"directory to store keys in"`
	Debug                     bool          `long:"debug" description:"flag to print debug output"`
	Timeout                   time.Duration `long:"timeout" description:"client timeout when doing queries"`
	BlockTimeout              time.Duration `long:"block-timeout" description:"block timeout when waiting for block events"`
	OutputFormat              string        `long:"output-format" description:"default output when printint responses"`
	SignModeStr               string        `long:"sign-mode" description:"sign mode to use"`
	BabylonContractAddress    string        `long:"babylon-contract-address" description:"address of the Babylon contract"`
	BtcStakingContractAddress string        `long:"btc-staking-contract-address" description:"address of the BTC staking contract"`
}

func DefaultCosmwasmConfig() *CosmwasmConfig {
	return &CosmwasmConfig{
		Key:                       "validator",
		ChainID:                   "wasmd-test",
		RPCAddr:                   "http://localhost:2990",
		GRPCAddr:                  "https://localhost:9090",
		AccountPrefix:             "wasm",
		KeyringBackend:            "test",
		GasAdjustment:             1.3,
		GasPrices:                 "1ustake",
		Debug:                     true,
		Timeout:                   20 * time.Second,
		BlockTimeout:              1 * time.Minute,
		OutputFormat:              "direct",
		SignModeStr:               "",
		BtcStakingContractAddress: "",
		BabylonContractAddress:    "",
	}
}

func ToQueryClientConfig(wc *CosmwasmConfig) *config.CosmwasmConfig {
	return &config.CosmwasmConfig{
		Key:                       wc.Key,
		ChainID:                   wc.ChainID,
		RPCAddr:                   wc.RPCAddr,
		AccountPrefix:             wc.AccountPrefix,
		KeyringBackend:            wc.KeyringBackend,
		GasAdjustment:             wc.GasAdjustment,
		GasPrices:                 wc.GasPrices,
		KeyDirectory:              wc.KeyDirectory,
		Debug:                     wc.Debug,
		Timeout:                   wc.Timeout,
		BlockTimeout:              wc.BlockTimeout,
		OutputFormat:              wc.OutputFormat,
		SignModeStr:               wc.SignModeStr,
		SubmitterAddress:          "",
		BabylonContractAddress:    wc.BabylonContractAddress,
		BtcStakingContractAddress: wc.BtcStakingContractAddress,
	}
}
