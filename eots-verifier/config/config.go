package config

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/babylonchain/finality-provider/util"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/jessevdk/go-flags"
)

const (
	defaultLogLevel       = "debug"
	defaultLogFilename    = "eots-verifier.log"
	defaultConfigFileName = "eots-verifier.conf"

	defaultRpcListener   = "127.0.0.1:9528"
	defaultBabylonRPC    = "http://127.0.0.1:26657"
	defaultEotsAggRPC    = "http://127.0.0.1:9527"
	defaultRollupRPC     = "http://127.0.0.1:8545"
	defaultRollupChainId = "42069"
)

var (
	// DefaultDir the default EOTS verifier home directory:
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.eots-verifier on Linux
	//   ~/Library/Application Support/Eots-verifier on MacOS
	DefaultDir = btcutil.AppDataDir("eots-verifier", false)
)

// Config is used to configure the EOTS verifier
type Config struct {
	LogLevel      string `long:"loglevel" description:"logging level"`
	RpcListener   string `long:"rpclistener" description:"the listener for RPC connections, e.g., 127.0.0.1:1234"`
	BabylonRPC    string `long:"babylon_rpc" description:"connect to the Babylon RPC service"`
	EotsAggRPC    string `long:"eots_agg_rpc" description:"connect to the EOTS Aggregator RPC service"`
	RollupRPC     string `long:"rollup_rpc" description:"connect to the Rollup RPC service"`
	RollupChainID string
}

// LoadConfig loads the `conf.toml` config file from a given path
func LoadConfig(homePath string) (*Config, error) {
	cfgFile := ConfigFile(homePath)
	if !util.FileExists(cfgFile) {
		return nil, fmt.Errorf("specified config file does not exist in %s", cfgFile)
	}

	var cfg Config
	fileParser := flags.NewParser(&cfg, flags.Default)
	err := flags.NewIniParser(fileParser).ParseFile(cfgFile)
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (cfg *Config) Validate() error {
	_, err := net.ResolveTCPAddr("tcp", cfg.RpcListener)
	if err != nil {
		return fmt.Errorf("invalid RPC listener address %s, %w", cfg.RpcListener, err)
	}

	if cfg.RollupRPC == "" || cfg.EotsAggRPC == "" || cfg.BabylonRPC == "" {
		return fmt.Errorf("missing needed RPC URL for the verifier")
	}

	return nil
}

func ConfigFile(homePath string) string {
	return filepath.Join(homePath, defaultConfigFileName)
}

func LogFile(homePath string) string {
	return filepath.Join(homePath, defaultLogFilename)
}

func DefaultConfig() *Config {
	return DefaultConfigWithHomePath(DefaultDir)
}

func DefaultConfigWithHomePath(homePath string) *Config {
	cfg := &Config{
		LogLevel:    defaultLogLevel,
		RpcListener: defaultRpcListener,
		BabylonRPC:  defaultBabylonRPC,
		EotsAggRPC:  defaultEotsAggRPC,
		RollupRPC:   defaultRollupRPC,
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	return cfg
}
