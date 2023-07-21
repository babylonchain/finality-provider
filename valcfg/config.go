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

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/sirupsen/logrus"
)

const (
	defaultDataDirname    = "data"
	defaultLogLevel       = "info"
	defaultLogDirname     = "logs"
	defaultLogFilename    = "vald.log"
	DefaultRPCPort        = 15812
	defaultConfigFileName = "vald.conf"
	defaultRandomNum      = 100
	defaultRandomNumMax   = 1000
)

var (
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Vald on MacOS
	DefaultValdDir = btcutil.AppDataDir("vald", false)

	DefaultConfigFile = filepath.Join(DefaultValdDir, defaultConfigFileName)

	defaultDataDir = filepath.Join(DefaultValdDir, defaultDataDirname)
	defaultLogDir  = filepath.Join(DefaultValdDir, defaultLogDirname)
)

// Config is the main config for the vald cli command
type Config struct {
	DebugLevel   string `long:"debuglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	ValdDir      string `long:"validatorddir" description:"The base directory that contains validator's data, logs, configuration file, etc."`
	ConfigFile   string `long:"configfile" description:"Path to configuration file"`
	DataDir      string `long:"datadir" description:"The directory to store validator's data within"`
	LogDir       string `long:"logdir" description:"Directory to log output."`
	DumpCfg      bool   `long:"dumpcfg" description:"If config file does not exist, create it with current settings"`
	RandomNum    uint64 `long:"randomnum" description:"The number of Schnorr public randomness for each commitment"`
	RandomNumMax uint64 `long:"randomnummax" description:"The upper bound of the number of Schnorr public randomness for each commitment"`
	// TODO: create Jury specific config
	JuryMode bool `long:"jurymode" description:"If the program is running in Jury mode"`

	DatabaseConfig *DatabaseConfig `group:"databaseconfig" namespace:"databaserpcconfig"`

	BabylonConfig *BBNConfig `group:"babylon" namespace:"babylon"`

	JuryModeConfig *JuryConfig `group:"jury" namespace:"jury"`

	GRpcServerConfig *GRpcServerConfig

	RpcListeners []net.Addr
}

func DefaultConfig() Config {
	bbnCfg := DefaultBBNConfig()
	dbCfg := DefaultDatabaseConfig()
	juryCfg := DefaultJuryConfig()
	return Config{
		ValdDir:        DefaultValdDir,
		ConfigFile:     DefaultConfigFile,
		DataDir:        defaultDataDir,
		DebugLevel:     defaultLogLevel,
		LogDir:         defaultLogDir,
		DatabaseConfig: &dbCfg,
		BabylonConfig:  &bbnCfg,
		JuryModeConfig: &juryCfg,
		RandomNum:      defaultRandomNum,
		RandomNumMax:   defaultRandomNumMax,
	}
}

type PollerConfig struct {
	StartingHeight uint64
	BufferSize     uint32
}

func DefaulPollerConfig() PollerConfig {
	return PollerConfig{
		StartingHeight: 1,
		BufferSize:     1000,
	}
}

// usageError is an error type that signals a problem with the supplied flags.
type usageError struct {
	err error
}

// Error returns the error string.
//
// NOTE: This is part of the error interface.
func (u *usageError) Error() string {
	return u.err.Error()
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//  1. Start with a default config with sane settings
//  2. Pre-parse the command line to check for an alternative config file
//  3. Load configuration file overwriting defaults with any specified options
//  4. Parse CLI options and overwrite/add any specified options
func LoadConfig() (*Config, *logrus.Logger, error) {
	// Pre-parse the command line options to pick up an alternative config
	// file.
	preCfg := DefaultConfig()
	if _, err := flags.Parse(&preCfg); err != nil {
		return nil, nil, err
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)

	// If the config file path has not been modified by the user, then
	// we'll use the default config file path. However, if the user has
	// modified their default dir, then we should assume they intend to use
	// the config file within it.
	configFileDir := CleanAndExpandPath(preCfg.ValdDir)
	configFilePath := CleanAndExpandPath(preCfg.ConfigFile)
	switch {
	case configFileDir != DefaultValdDir &&
		configFilePath == DefaultConfigFile:

		configFilePath = filepath.Join(
			configFileDir, defaultConfigFileName,
		)

	// User did specify an explicit --configfile, so we check that it does
	// exist under that path to avoid surprises.
	case configFilePath != DefaultConfigFile:
		if !FileExists(configFilePath) {
			return nil, nil, fmt.Errorf("specified config file does "+
				"not exist in %s", configFilePath)
		}
	}

	// Next, load any additional configuration options from the file.
	var configFileError error
	cfg := preCfg
	fileParser := flags.NewParser(&cfg, flags.Default)
	err := flags.NewIniParser(fileParser).ParseFile(configFilePath)
	if err != nil {
		// If it's a parsing related error, then we'll return
		// immediately, otherwise we can proceed as possibly the config
		// file doesn't exist which is OK.
		if _, ok := err.(*flags.IniError); ok {
			return nil, nil, err
		}

		configFileError = err
	}

	// Finally, parse the remaining command line options again to ensure
	// they take precedence.
	flagParser := flags.NewParser(&cfg, flags.Default)
	if _, err := flagParser.Parse(); err != nil {
		return nil, nil, err
	}

	cfgLogger := logrus.New()
	cfgLogger.Out = os.Stdout
	// Make sure everything we just loaded makes sense.
	cleanCfg, err := ValidateConfig(cfg)
	if err != nil {
		// Log help message in case of usage error.
		if _, ok := err.(*usageError); ok {
			cfgLogger.Warnf("Incorrect usage: %v", usageMessage)
		}

		cfgLogger.Warnf("Error validating config: %v", err)
		return nil, nil, err
	}

	// ignore error here as we already validated the value
	logRuslLevel, _ := logrus.ParseLevel(cleanCfg.DebugLevel)

	// TODO: Add log rotation
	// At this point we know config is valid, create logger which also log to file
	logFilePath := filepath.Join(cleanCfg.LogDir, defaultLogFilename)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)

	cfgLogger.Out = mw
	cfgLogger.Level = logRuslLevel

	// Warn about missing config file only after all other configuration is
	// done. This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		cfgLogger.Warnf("%v", configFileError)
		if cleanCfg.DumpCfg {
			cfgLogger.Infof("Writing configuration file to %s", configFilePath)
			fileParser := flags.NewParser(&cfg, flags.Default)
			err := flags.NewIniParser(fileParser).WriteFile(configFilePath, flags.IniIncludeComments|flags.IniIncludeDefaults)
			if err != nil {
				cfgLogger.Warnf("Error writing configuration file: %v", err)
				return nil, nil, err
			}
		}
	}

	return cleanCfg, cfgLogger, nil
}

// ValidateConfig check the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func ValidateConfig(cfg Config) (*Config, error) {
	// If the provided stakerd directory is not the default, we'll modify the
	// path to all the files and directories that will live within it.
	valdDir := CleanAndExpandPath(cfg.ValdDir)
	if valdDir != DefaultValdDir {
		cfg.DataDir = filepath.Join(valdDir, defaultDataDirname)
		cfg.LogDir = filepath.Join(valdDir, defaultLogDirname)
	}

	funcName := "ValidateConfig"
	mkErr := func(format string, args ...interface{}) error {
		return fmt.Errorf(funcName+": "+format, args...)
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

			str := "Failed to create vald directory '%s': %v"
			return mkErr(str, dir, err)
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
	if cfg.JuryMode {
		switch cfg.JuryModeConfig.BitcoinNetwork {
		case "testnet":
			cfg.JuryModeConfig.ActiveNetParams = chaincfg.TestNet3Params
		case "regtest":
			cfg.JuryModeConfig.ActiveNetParams = chaincfg.RegressionNetParams
		case "simnet":
			cfg.JuryModeConfig.ActiveNetParams = chaincfg.SimNetParams
		case "signet":
			cfg.JuryModeConfig.ActiveNetParams = chaincfg.SigNetParams
		default:
			return nil, mkErr(fmt.Sprintf("invalid network: %v",
				cfg.JuryModeConfig.BitcoinNetwork))
		}
	}

	// Create the vald directory and all other subdirectories if they
	// don't already exist. This makes sure that directory trees are also
	// created for files that point to outside the vald dir.
	dirs := []string{
		valdDir, cfg.DataDir, cfg.LogDir,
	}
	for _, dir := range dirs {
		if err := makeDirectory(dir); err != nil {
			return nil, err
		}
	}

	// At least one RPCListener is required. So listen on localhost per
	// default.
	if len(cfg.GRpcServerConfig.RawRPCListeners) == 0 {
		addr := fmt.Sprintf("localhost:%d", DefaultRPCPort)
		cfg.GRpcServerConfig.RawRPCListeners = append(
			cfg.GRpcServerConfig.RawRPCListeners, addr,
		)
	}

	_, err := logrus.ParseLevel(cfg.DebugLevel)

	if err != nil {
		return nil, mkErr("error parsing debuglevel: %v", err)
	}

	// Add default port to all RPC listener addresses if needed and remove
	// duplicate addresses.
	cfg.RpcListeners, err = lncfg.NormalizeAddresses(
		cfg.GRpcServerConfig.RawRPCListeners, strconv.Itoa(DefaultRPCPort),
		net.ResolveTCPAddr,
	)

	if err != nil {
		return nil, mkErr("error normalizing RPC listen addrs: %v", err)
	}

	// All good, return the sanitized result.
	return &cfg, nil
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

type GRpcServerConfig struct {
	RawRPCListeners []string `long:"rpclisten" description:"Add an interface/port/socket to listen for RPC connections"`
}
