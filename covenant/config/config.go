package config

import (
	"fmt"
	"github.com/babylonchain/btc-validator/util"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jessevdk/go-flags"

	"github.com/babylonchain/btc-validator/config"
	"github.com/babylonchain/btc-validator/log"
)

const (
	defaultLogLevel        = "debug"
	defaultLogFilename     = "covd.log"
	defaultConfigFileName  = "covd.conf"
	defaultCovenantKeyName = "covenant-key"
	defaultQueryInterval   = 15 * time.Second
	defaultDelegationLimit = uint64(100)
	defaultBitcoinNetwork  = "simnet"
	defaultLogDirname      = "logs"
)

var (
	// DefaultCovenantDir specifies the default home directory for the covenant:
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Covd on MacOS
	DefaultCovenantDir = btcutil.AppDataDir("covd", false)

	defaultBTCNetParams = chaincfg.SimNetParams
)

type Config struct {
	LogLevel        string        `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	QueryInterval   time.Duration `long:"queryinterval" description:"The interval between each query for pending BTC delegations"`
	DelegationLimit uint64        `long:"delegationlimit" description:"The maximum number of delegations that the Covenant processes each time"`
	BitcoinNetwork  string        `long:"bitcoinnetwork" description:"Bitcoin network to run on" choice:"mainnet" choice:"regtest" choice:"testnet" choice:"simnet" choice:"signet"`

	BTCNetParams chaincfg.Params

	BabylonConfig *config.BBNConfig `group:"babylon" namespace:"babylon"`
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//  1. Start with a default config with sane settings
//  2. Pre-parse the command line to check for an alternative config file
//  3. Load configuration file overwriting defaults with any specified options
//  4. Parse CLI options and overwrite/add any specified options
func LoadConfig(homePath string) (*Config, *zap.Logger, error) {
	// The home directory is required to have a configuration file with a specific name
	// under it.
	homePath = util.CleanAndExpandPath(homePath)
	cfgFile := ConfigFile(homePath)
	if !util.FileExists(cfgFile) {
		return nil, nil, fmt.Errorf("specified config file does "+
			"not exist in %s", cfgFile)
	}

	// If there are issues parsing the config file, return an error
	var cfg Config
	fileParser := flags.NewParser(&cfg, flags.Default)
	err := flags.NewIniParser(fileParser).ParseFile(cfgFile)
	if err != nil {
		return nil, nil, err
	}

	// Make sure everything we just loaded makes sense.
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	// Initialize the logger basecd on the configuration
	logger, err := initLogger(homePath, cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	return &cfg, logger, nil
}

// Validate check the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func (cfg *Config) Validate() error {
	switch cfg.BitcoinNetwork {
	case "mainnet":
		cfg.BTCNetParams = chaincfg.MainNetParams
	case "testnet":
		cfg.BTCNetParams = chaincfg.TestNet3Params
	case "regtest":
		cfg.BTCNetParams = chaincfg.RegressionNetParams
	case "simnet":
		cfg.BTCNetParams = chaincfg.SimNetParams
	case "signet":
		cfg.BTCNetParams = chaincfg.SigNetParams
	default:
		return fmt.Errorf("unsupported Bitcoin network: %s", cfg.BitcoinNetwork)
	}

	return nil
}

func ConfigFile(homePath string) string {
	return filepath.Join(homePath, defaultConfigFileName)
}

func LogDir(homePath string) string {
	return filepath.Join(homePath, defaultLogDirname)
}

func LogFile(homePath string) string {
	return filepath.Join(LogDir(homePath), defaultLogFilename)
}

func initLogger(homePath string, logLevel string) (*zap.Logger, error) {
	if err := util.MakeDirectory(LogDir(homePath)); err != nil {
		return nil, err
	}
	logFilePath := LogFile(homePath)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)

	logger, err := log.NewRootLogger("console", logLevel, mw)
	if err != nil {
		return nil, err
	}
	return logger, nil
}

func DefaultConfigWithHomePath(homePath string) Config {
	bbnCfg := config.DefaultBBNConfig()
	bbnCfg.Key = defaultCovenantKeyName
	bbnCfg.KeyDirectory = homePath
	cfg := Config{
		LogLevel:        defaultLogLevel,
		QueryInterval:   defaultQueryInterval,
		DelegationLimit: defaultDelegationLimit,
		BitcoinNetwork:  defaultBitcoinNetwork,
		BTCNetParams:    defaultBTCNetParams,
		BabylonConfig:   &bbnCfg,
	}

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	return cfg
}

func DefaultConfig() Config {
	return DefaultConfigWithHomePath(DefaultCovenantDir)
}
