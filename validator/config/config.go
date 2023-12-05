package valcfg

import (
	"fmt"
	"github.com/babylonchain/btc-validator/util"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/config"
	"github.com/babylonchain/btc-validator/log"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jessevdk/go-flags"

	eotscfg "github.com/babylonchain/btc-validator/eotsmanager/config"
)

const (
	defaultChainName                      = "babylon"
	defaultLogLevel                       = "info"
	defaultLogDirname                     = "logs"
	defaultLogFilename                    = "vald.log"
	defaultValidatorKeyName               = "btc-validator"
	DefaultRPCPort                        = 15812
	defaultConfigFileName                 = "vald.conf"
	defaultNumPubRand                     = 100
	defaultNumPubRandMax                  = 100
	defaultMinRandHeightGap               = 10
	defaultStatusUpdateInterval           = 5 * time.Second
	defaultRandomInterval                 = 5 * time.Second
	defautlUnbondingSigSubmissionInterval = 20 * time.Second
	defaultSubmitRetryInterval            = 1 * time.Second
	defaultFastSyncInterval               = 20 * time.Second
	defaultFastSyncLimit                  = 10
	defaultFastSyncGap                    = 6
	defaultMaxSubmissionRetries           = 20
	defaultBitcoinNetwork                 = "simnet"
)

var (
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Vald on MacOS
	DefaultValdDir = btcutil.AppDataDir("vald", false)

	defaultActiveNetParams    = chaincfg.SimNetParams
	defaultEOTSManagerAddress = "127.0.0.1:" + strconv.Itoa(eotscfg.DefaultRPCPort)
	defaultRpcListener        = "localhost:" + strconv.Itoa(DefaultRPCPort)
)

// Config is the main config for the vald cli command
type Config struct {
	LogLevel string `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	// ChainName and ChainID (if any) of the chain config identify a consumer chain
	ChainName                      string        `long:"chainname" description:"the name of the consumer chain" choice:"babylon"`
	NumPubRand                     uint64        `long:"numPubRand" description:"The number of Schnorr public randomness for each commitment"`
	NumPubRandMax                  uint64        `long:"numpubrandmax" description:"The upper bound of the number of Schnorr public randomness for each commitment"`
	MinRandHeightGap               uint64        `long:"minrandheightgap" description:"The minimum gap between the last committed rand height and the current Babylon block height"`
	StatusUpdateInterval           time.Duration `long:"statusupdateinterval" description:"The interval between each update of validator status"`
	RandomnessCommitInterval       time.Duration `long:"randomnesscommitinterval" description:"The interval between each attempt to commit public randomness"`
	SubmissionRetryInterval        time.Duration `long:"submissionretryinterval" description:"The interval between each attempt to submit finality signature or public randomness after a failure"`
	UnbondingSigSubmissionInterval time.Duration `long:"unbondingsigsubmissioninterval" description:"The interval between each attempt to check and submit unbonding signature"`
	MaxSubmissionRetries           uint64        `long:"maxsubmissionretries" description:"The maximum number of retries to submit finality signature or public randomness"`
	FastSyncInterval               time.Duration `long:"fastsyncinterval" description:"The interval between each try of fast sync, which is disabled if the value is 0"`
	FastSyncLimit                  uint64        `long:"fastsynclimit" description:"The maximum number of blocks to catch up for each fast sync"`
	FastSyncGap                    uint64        `long:"fastsyncgap" description:"The block gap that will trigger the fast sync"`
	EOTSManagerAddress             string        `long:"eotsmanageraddress" description:"The address of the remote EOTS manager; Empty if the EOTS manager is running locally"`

	BitcoinNetwork string `long:"bitcoinnetwork" description:"Bitcoin network to run on" choice:"regtest" choice:"testnet" choice:"simnet" choice:"signet"`

	ActiveNetParams chaincfg.Params

	PollerConfig *ChainPollerConfig `group:"chainpollerconfig" namespace:"chainpollerconfig"`

	DatabaseConfig *DatabaseConfig `group:"databaseconfig" namespace:"databaseconfig"`

	EOTSManagerConfig *EOTSManagerConfig `group:"eotsmanagerconfig" namespace:"eotsmanagerconfig"`

	BabylonConfig *config.BBNConfig `group:"babylon" namespace:"babylon"`

	ValidatorModeConfig *ValidatorConfig `group:"validator" namespace:"validator"`

	RpcListener string `long:"rpclistener" description:"the listener for RPC connections, e.g., localhost:1234"`
}

func DefaultConfigWithHome(homePath string) Config {
	bbnCfg := config.DefaultBBNConfig()
	bbnCfg.Key = defaultValidatorKeyName
	bbnCfg.KeyDirectory = homePath
	dbCfg := DefaultDatabaseConfig()
	pollerCfg := DefaultChainPollerConfig()
	valCfg := DefaultValidatorConfig()
	eotsMngrCfg := DefaultEOTSManagerConfig()
	cfg := Config{
		ChainName:                      defaultChainName,
		LogLevel:                       defaultLogLevel,
		DatabaseConfig:                 &dbCfg,
		BabylonConfig:                  &bbnCfg,
		ValidatorModeConfig:            &valCfg,
		PollerConfig:                   &pollerCfg,
		EOTSManagerConfig:              &eotsMngrCfg,
		NumPubRand:                     defaultNumPubRand,
		NumPubRandMax:                  defaultNumPubRandMax,
		MinRandHeightGap:               defaultMinRandHeightGap,
		StatusUpdateInterval:           defaultStatusUpdateInterval,
		RandomnessCommitInterval:       defaultRandomInterval,
		SubmissionRetryInterval:        defaultSubmitRetryInterval,
		UnbondingSigSubmissionInterval: defautlUnbondingSigSubmissionInterval,
		FastSyncInterval:               defaultFastSyncInterval,
		FastSyncLimit:                  defaultFastSyncLimit,
		FastSyncGap:                    defaultFastSyncGap,
		MaxSubmissionRetries:           defaultMaxSubmissionRetries,
		BitcoinNetwork:                 defaultBitcoinNetwork,
		ActiveNetParams:                defaultActiveNetParams,
		EOTSManagerAddress:             defaultEOTSManagerAddress,
		RpcListener:                    defaultRpcListener,
	}

	_ = cfg.Validate()

	return cfg
}

func DefaultConfig() Config {
	return DefaultConfigWithHome(DefaultValdDir)
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

func NewEOTSManagerConfigFromAppConfig(appCfg *Config) (*eotscfg.Config, error) {
	dbCfg, err := eotscfg.NewDatabaseConfig(
		appCfg.EOTSManagerConfig.DBBackend,
		appCfg.EOTSManagerConfig.DBPath,
		appCfg.EOTSManagerConfig.DBName,
	)
	if err != nil {
		return nil, err
	}
	return &eotscfg.Config{
		LogLevel:       appCfg.LogLevel,
		KeyDirectory:   appCfg.BabylonConfig.KeyDirectory,
		KeyringBackend: appCfg.BabylonConfig.KeyringBackend,
		DatabaseConfig: dbCfg,
	}, nil
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
	cfgFile := ConfigFile(homePath)
	if !util.FileExists(cfgFile) {
		return nil, nil, fmt.Errorf("specified config file does "+
			"not exist in %s", cfgFile)
	}

	// Next, load any additional configuration options from the file.
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

	logger, err := initLogger(homePath, cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	return &cfg, logger, nil
}

// Validate checks the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func (cfg *Config) Validate() error {
	// Multiple networks can't be selected simultaneously.  Count number of
	// network flags passed; assign active network params
	// while we're at it.

	switch cfg.BitcoinNetwork {
	case "testnet":
		cfg.ActiveNetParams = chaincfg.TestNet3Params
	case "regtest":
		cfg.ActiveNetParams = chaincfg.RegressionNetParams
	case "simnet":
		cfg.ActiveNetParams = chaincfg.SimNetParams
	case "signet":
		cfg.ActiveNetParams = chaincfg.SigNetParams
	default:
		return fmt.Errorf("invalid network: %v", cfg.BitcoinNetwork)
	}

	_, err := net.ResolveTCPAddr("tcp", cfg.RpcListener)
	if err != nil {
		return fmt.Errorf("invalid RPC listener address %s, %w", cfg.RpcListener, err)
	}

	// All good, return the sanitized result.
	return nil
}
