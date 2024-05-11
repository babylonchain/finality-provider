package keymanager

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
)

var _ KeyManager = &EVMKeyManager{}

type EVMKeyManager struct {
	Keystore *keystore.KeyStore
	Logger   *zap.Logger
}

func (evmKey *EVMKeyManager) CreateKeyPair(passphrase string) (string, error) {
	account, err := evmKey.Keystore.NewAccount(passphrase)
	if err != nil {
		return "", fmt.Errorf("failed to create an EVM key: %v", err)
	}

	evmKey.Logger.Info(
		"successfully created an EVM key",
		zap.String("address", account.Address.Hex()),
	)
	return account.Address.Hex(), nil
}

func (evmKey *EVMKeyManager) GetPrivkey(addr, passphrase string) (string, error) {
	account := accounts.Account{Address: common.HexToAddress(addr)}
	keyjson, err := evmKey.Keystore.Export(account, passphrase, passphrase)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve an EVM key: %v", err)
	}

	key, err := keystore.DecryptKey(keyjson, passphrase)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt the EVM key: %v", err)
	}

	privkey := hex.EncodeToString(crypto.FromECDSA(key.PrivateKey))

	return privkey, nil
}
