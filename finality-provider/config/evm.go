package config

import (
	"os"
	"path/filepath"

	"github.com/babylonchain/finality-provider/keymanager"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"go.uber.org/zap"
)

const (
	defaultEVMRPCAddr = "http://127.0.0.1:8545"
)

type EVMConfig struct {
	RPCAddr      string `long:"rpc-address" description:"address of the rpc server to connect to"`
	KeyDirectory string `long:"key-dir" description:"directory to store keys in"`
	Passphrase   string `long:"passphrase" description:"specifies the password used to encrypt the key"`
	Address      string `long:"address" description:"the address to sign transactions with"`
}

func DefaultEVMConfig() EVMConfig {
	return EVMConfig{
		RPCAddr:      defaultEVMRPCAddr,
		KeyDirectory: defaultEVMKeyHome(),
		Passphrase:   "",
		Address:      "",
	}
}

func defaultEVMKeyHome() string {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(userHomeDir, ".evmkeystore")
}

func (cfg *EVMConfig) SetPassphrase(passphrase string) {
	cfg.Passphrase = passphrase
}

func (cfg *EVMConfig) SetAddress(address string) {
	cfg.Address = address
}

func (cfg *EVMConfig) NewEVMKeyManager(logger *zap.Logger) *keymanager.EVMKeyManager {
	ks := keystore.NewKeyStore(cfg.KeyDirectory, keystore.StandardScryptN, keystore.StandardScryptP)
	return &keymanager.EVMKeyManager{
		Keystore: ks,
		Logger:   logger,
	}
}
