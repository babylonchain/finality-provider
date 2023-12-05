package valcfg

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
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
	defaultDataDirname                    = "data"
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

	DefaultConfigFile = filepath.Join(DefaultValdDir, defaultConfigFileName)

	defaultDataDir            = filepath.Join(DefaultValdDir, defaultDataDirname)
	defaultLogDir             = filepath.Join(DefaultValdDir, defaultLogDirname)
	defaultActiveNetParams    = chaincfg.SimNetParams
	defaultEOTSManagerAddress = "127.0.0.1:" + strconv.Itoa(eotscfg.DefaultRPCPort)
	defaultRpcListener        = "localhost:" + strconv.Itoa(DefaultRPCPort)
)

// Config is the main config for the vald cli command
type Config struct {
	LogLevel string `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	// ChainName and ChainID (if any) of the chain config identify a consumer chain
	ChainName                      string        `long:"chainname" description:"the name of the consumer chain" choice:"babylon"`
	ValdDir                        string        `long:"validatorddir" description:"The base directory that contains validator's data, logs, configuration file, etc."`
	ConfigFile                     string        `long:"configfile" description:"Path to configuration file"`
	DataDir                        string        `long:"datadir" description:"The directory to store validator's data within"`
	LogDir                         string        `long:"logdir" description:"Directory to log output."`
	DumpCfg                        bool          `long:"dumpcfg" description:"If config file does not exist, create it with current settings"`
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

func DefaultConfig() Config {
	bbnCfg := config.DefaultBBNConfig()
	bbnCfg.Key = defaultValidatorKeyName
	bbnCfg.KeyDirectory = DefaultValdDir
	dbCfg := DefaultDatabaseConfig()
	pollerCfg := DefaultChainPollerConfig()
	valCfg := DefaultValidatorConfig()
	eotsMngrCfg := DefaultEOTSManagerConfig()
	cfg := Config{
		ValdDir:                        DefaultValdDir,
		ChainName:                      defaultChainName,
		ConfigFile:                     DefaultConfigFile,
		DataDir:                        defaultDataDir,
		LogLevel:                       defaultLogLevel,
		LogDir:                         defaultLogDir,
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
		EOTSDir:        appCfg.ValdDir,
		ConfigFile:     appCfg.ConfigFile,
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
func LoadConfig(filePath string) (*Config, *zap.Logger, error) {
	// Pre-parse the command line options to pick up an alternative config
	// file.
	preCfg := DefaultConfig()
	if _, err := flags.Parse(&preCfg); err != nil {
		return nil, nil, err
	}

	if !FileExists(filePath) {
		return nil, nil, fmt.Errorf("specified config file does "+
			"not exist in %s", filePath)
	}

	// Next, load any additional configuration options from the file.
	var configFileError error
	cfg := preCfg
	fileParser := flags.NewParser(&cfg, flags.Default)
	err := flags.NewIniParser(fileParser).ParseFile(filePath)
	if err != nil {
		// If it's a parsing related error, then we'll return
		// immediately, otherwise we can proceed as possibly the config
		// file doesn't exist which is OK.
		if _, ok := err.(*flags.IniError); ok {
			return nil, nil, err
		}

		configFileError = err
	}

	// Make sure everything we just loaded makes sense.
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	// At this point we know config is valid, create logger which also log to file
	logFilePath := filepath.Join(cfg.LogDir, defaultLogFilename)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)

	cfgLogger, err := log.NewRootLogger("console", cfg.LogLevel, mw)
	if err != nil {
		return nil, nil, err
	}

	// Warn about missing config file only after all other configuration is
	// done. This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		if cfg.DumpCfg {
			cfgLogger.Info("Writing configuration file", zap.String("path", filePath))
			fileParser := flags.NewParser(&cfg, flags.Default)
			err := flags.NewIniParser(fileParser).WriteFile(filePath, flags.IniIncludeComments|flags.IniIncludeDefaults)
			if err != nil {
				cfgLogger.Error("Error writing configuration file", zap.Error(err))
				return nil, nil, err
			}
		}
	}

	return &cfg, cfgLogger, nil
}

// Validate checks the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func (cfg *Config) Validate() error {
	// If the provided stakerd directory is not the default, we'll modify the
	// path to all the files and directories that will live within it.
	valdDir := CleanAndExpandPath(cfg.ValdDir)
	if valdDir != DefaultValdDir {
		cfg.DataDir = filepath.Join(valdDir, defaultDataDirname)
		cfg.LogDir = filepath.Join(valdDir, defaultLogDirname)
	}

	makeDirectory := func(dir string) error {
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			// Show a nicer error message if it's because a symlink
			// is linked to a directory that does not exist
			// (probably because it's not mounted).
			if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
				link, lerr := os.Readlink(e.Path)
				if lerr == nil {
					str := "is symlink %s -> %s mounted?"
					err = fmt.Errorf(str, e.Path, link)
				}
			}

			return fmt.Errorf("failed to create dir %s: %w", dir, err)
		}

		return nil
	}

	// As soon as we're done parsing configuration options, ensure all
	// paths to directories and files are cleaned and expanded before
	// attempting to use them later on.
	cfg.DataDir = CleanAndExpandPath(cfg.DataDir)
	cfg.LogDir = CleanAndExpandPath(cfg.LogDir)

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

	// Create the vald directory and all other subdirectories if they
	// don't already exist. This makes sure that directory trees are also
	// created for files that point to outside the vald dir.
	dirs := []string{
		valdDir, cfg.DataDir, cfg.LogDir,
	}
	for _, dir := range dirs {
		if err := makeDirectory(dir); err != nil {
			return err
		}
	}

	_, err := net.ResolveTCPAddr("tcp", cfg.RpcListener)
	if err != nil {
		return fmt.Errorf("invalid RPC listener address %s, %w", cfg.RpcListener, err)
	}

	// All good, return the sanitized result.
	return nil
}

// FileExists reports whether the named file or directory exists.
// This function is taken from https://github.com/btcsuite/btcd
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// CleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func CleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		u, err := user.Current()
		if err == nil {
			homeDir = u.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}
