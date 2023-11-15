package config

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/99designs/keyring"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
)

const (
	defaultLogLevel       = "debug"
	defaultDataDirname    = "data"
	defaultLogDirname     = "logs"
	defaultLogFilename    = "eotsd.log"
	defaultConfigFileName = "eotsd.conf"
	DefaultRPCPort        = 15813
	defaultKeyringBackend = string(keyring.FileBackend)
)

var (
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Eotsd on MacOS
	DefaultEOTSDir = btcutil.AppDataDir("eotsd", false)

	DefaultConfigFile = filepath.Join(DefaultEOTSDir, defaultConfigFileName)

	defaultDataDir     = filepath.Join(DefaultEOTSDir, defaultDataDirname)
	defaultLogDir      = filepath.Join(DefaultEOTSDir, defaultLogDirname)
	defaultRpcListener = "localhost:" + strconv.Itoa(DefaultRPCPort)
)

type Config struct {
	LogLevel       string `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
	EOTSDir        string `long:"workdir" description:"The base directory that contains the EOTS manager's data, logs, configuration file, etc."`
	ConfigFile     string `long:"configfile" description:"Path to configuration file"`
	DataDir        string `long:"datadir" description:"The directory to store validator's data within"`
	LogDir         string `long:"logdir" description:"Directory to log output."`
	DumpCfg        bool   `long:"dumpcfg" description:"If config file does not exist, create it with current settings"`
	KeyDirectory   string `long:"key-dir" description:"Directory to store keys in"`
	KeyringBackend string `long:"keyring-type" description:"Type of keyring to use"`

	DatabaseConfig *DatabaseConfig

	RpcListener string `long:"rpclistener" description:"the listener for RPC connections, e.g., localhost:1234"`
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

	// If the config file path has not been modified by the user, then
	// we'll use the default config file path. However, if the user has
	// modified their default dir, then we should assume they intend to use
	// the config file within it.
	configFileDir := cleanAndExpandPath(preCfg.EOTSDir)
	configFilePath := cleanAndExpandPath(preCfg.ConfigFile)
	switch {
	case configFileDir != DefaultEOTSDir &&
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
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	logRuslLevel, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	// TODO: Add log rotation
	// At this point we know config is valid, create logger which also log to file
	logFilePath := filepath.Join(cfg.LogDir, defaultLogFilename)
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
			cfgLogger.Infof("Writing configuration file to %s", configFilePath)
			fileParser := flags.NewParser(&cfg, flags.Default)
			err := flags.NewIniParser(fileParser).WriteFile(configFilePath, flags.IniIncludeComments|flags.IniIncludeDefaults)
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
	// If the provided eotsd directory is not the default, we'll modify the
	// path to all the files and directories that will live within it.
	eotsdDir := cleanAndExpandPath(cfg.EOTSDir)
	if eotsdDir != DefaultEOTSDir {
		cfg.DataDir = filepath.Join(eotsdDir, defaultDataDirname)
		cfg.LogDir = filepath.Join(eotsdDir, defaultLogDirname)
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

			return fmt.Errorf("failed to create eotsd directory '%s': %w", dir, err)
		}

		return nil
	}

	// As soon as we're done parsing configuration options, ensure all
	// paths to directories and files are cleaned and expanded before
	// attempting to use them later on.
	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)

	// Create the eotsd directory and all other subdirectories if they
	// don't already exist. This makes sure that directory trees are also
	// created for files that point to outside the eotsd dir.
	dirs := []string{
		eotsdDir, cfg.DataDir, cfg.LogDir,
	}
	for _, dir := range dirs {
		if err := makeDirectory(dir); err != nil {
			return err
		}
	}

	_, err := logrus.ParseLevel(cfg.LogLevel)

	if err != nil {
		return err
	}

	_, err = net.ResolveTCPAddr("tcp", cfg.RpcListener)
	if err != nil {
		return fmt.Errorf("invalid RPC listener address %s, %w", cfg.RpcListener, err)
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

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
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

func DefaultConfig() Config {
	dbCfg := DefaultDatabaseConfig()
	return Config{
		LogLevel:       defaultLogLevel,
		EOTSDir:        DefaultEOTSDir,
		ConfigFile:     DefaultConfigFile,
		DataDir:        defaultDataDir,
		LogDir:         defaultLogDir,
		KeyringBackend: defaultKeyringBackend,
		KeyDirectory:   defaultDataDir,
		DatabaseConfig: &dbCfg,
		RpcListener:    defaultRpcListener,
	}
}
