package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	defaultLogLevel        = "debug"
	defaultLogFilename     = "covd.log"
	defaultConfigFileName  = "covd.conf"
	defaultCovenantKeyName = "covenant-key"
	defaultQueryInterval   = 15 * time.Second
	defaultDelegationLimit = uint64(100)
	defaultBitcoinNetwork  = "simnet"
	emptySlashingAddress   = ""
)

var (
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Covd on MacOS
	DefaultCovenantDir = btcutil.AppDataDir("covd", false)

	DefaultConfigFile = filepath.Join(DefaultCovenantDir, defaultConfigFileName)
)

type Config struct {
	LogLevel        string        `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	CovenantDir     string        `long:"covenantdir" description:"The base directory of the Covenant emulator"`
	DumpCfg         bool          `long:"dumpcfg" description:"If config file does not exist, create it with current settings"`
	QueryInterval   time.Duration `long:"queryinterval" description:"The interval between each query for pending BTC delegations"`
	DelegationLimit uint64        `long:"delegationlimit" description:"The maximum number of delegations that the Covenant processes each time"`
	// slashing address can be empty as it will be set from Babylon params
	SlashingAddress string `long:"slashingaddress" description:"The slashing address that the slashed fund is sent to"`
	BitcoinNetwork  string `long:"bitcoinnetwork" description:"Bitcoin network to run on" choice:"regtest" choice:"testnet" choice:"simnet" choice:"signet"`

	ActiveNetParams chaincfg.Params

	BabylonConfig *valcfg.BBNConfig `group:"babylon" namespace:"babylon"`
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//  1. Start with a default config with sane settings
//  2. Pre-parse the command line to check for an alternative config file
//  3. Load configuration file overwriting defaults with any specified options
//  4. Parse CLI options and overwrite/add any specified options
func LoadConfig(filePath string) (*Config, *logrus.Logger, error) {
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

	cfgLogger := logrus.New()
	cfgLogger.Out = os.Stdout
	// Make sure everything we just loaded makes sense.
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	logRuslLevel, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	// At this point we know config is valid, create logger which also log to file
	logFilePath := filepath.Join(cfg.CovenantDir, defaultLogFilename)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)

	cfgLogger.SetOutput(mw)
	cfgLogger.SetLevel(logRuslLevel)

	// Warn about missing config file only after all other configuration is
	// done. This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		cfgLogger.Warnf("%v", configFileError)
		if cfg.DumpCfg {
			cfgLogger.Infof("Writing configuration file to %s", filePath)
			fileParser := flags.NewParser(&cfg, flags.Default)
			err := flags.NewIniParser(fileParser).WriteFile(filePath, flags.IniIncludeComments|flags.IniIncludeDefaults)
			if err != nil {
				cfgLogger.Warnf("Error writing configuration file: %v", err)
				return nil, nil, err
			}
		}
	}

	return &cfg, cfgLogger, nil
}

// Validate check the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func (cfg *Config) Validate() error {
	err := os.MkdirAll(cfg.CovenantDir, 0700)
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

		return fmt.Errorf("failed to create covd directory '%s': %w", cfg.CovenantDir, err)
	}

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
		return fmt.Errorf("unsupported Bitcoin network: %s", cfg.BitcoinNetwork)
	}

	_, err = logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return err
	}

	return nil
}

func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func DefaultConfig() Config {
	bbnCfg := valcfg.DefaultBBNConfig()
	bbnCfg.Key = defaultCovenantKeyName
	bbnCfg.KeyDirectory = DefaultCovenantDir
	return Config{
		LogLevel:        defaultLogLevel,
		CovenantDir:     DefaultCovenantDir,
		DumpCfg:         false,
		QueryInterval:   defaultQueryInterval,
		DelegationLimit: defaultDelegationLimit,
		SlashingAddress: emptySlashingAddress,
		BitcoinNetwork:  defaultBitcoinNetwork,
		ActiveNetParams: chaincfg.Params{},
		BabylonConfig:   &bbnCfg,
	}
}
