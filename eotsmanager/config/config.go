package config

import (
	"fmt"
	"github.com/babylonchain/btc-validator/log"
	"github.com/babylonchain/btc-validator/util"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const (
	defaultLogLevel       = "debug"
	defaultLogDirname     = "logs"
	defaultLogFilename    = "eotsd.log"
	defaultConfigFileName = "eotsd.conf"
	DefaultRPCPort        = 15813
	defaultKeyringBackend = keyring.BackendFile
)

var (
	// DefaultEOTSDir the default EOTS home directory:
	//   C:\Users\<username>\AppData\Local\ on Windows
	//   ~/.vald on Linux
	//   ~/Library/Application Support/Eotsd on MacOS
	DefaultEOTSDir = btcutil.AppDataDir("eotsd", false)

	defaultRpcListener = "localhost:" + strconv.Itoa(DefaultRPCPort)
)

type Config struct {
	LogLevel       string `long:"loglevel" description:"Logging level for all subsystems" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal"`
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
func LoadConfig(homePath string) (*Config, *zap.Logger, error) {
	// The home directory is required to have a configuration file with a specific name
	// under it.
	homePath = util.CleanAndExpandPath(homePath)
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

// Validate check the given configuration to be sane. This makes sure no
// illegal values or combination of values are set. All file system paths are
// normalized. The cleaned up config is returned on success.
func (cfg *Config) Validate() error {
	_, err := net.ResolveTCPAddr("tcp", cfg.RpcListener)
	if err != nil {
		return fmt.Errorf("invalid RPC listener address %s, %w", cfg.RpcListener, err)
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
	// TODO: Add log rotation
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

func DefaultConfigWithHome(homePath string) Config {
	dbCfg := DefaultDatabaseConfig()
	cfg := Config{
		LogLevel:       defaultLogLevel,
		KeyringBackend: defaultKeyringBackend,
		KeyDirectory:   homePath,
		DatabaseConfig: &dbCfg,
		RpcListener:    defaultRpcListener,
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	return cfg
}
func DefaultConfig() Config {
	return DefaultConfigWithHome(DefaultEOTSDir)
}
