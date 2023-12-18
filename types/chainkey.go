package types

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	fpkr "github.com/babylonchain/finality-provider/keyring"
)

type ChainKeyInfo struct {
	Name       string
	Mnemonic   string
	PublicKey  *btcec.PublicKey
	PrivateKey *btcec.PrivateKey
}

func (ki *ChainKeyInfo) GetValAddress() (sdk.ValAddress, error) {
	if ki.PublicKey == nil {
		return nil, fmt.Errorf("empty public key")
	}
	pk := secp256k1.PubKey{Key: ki.PublicKey.SerializeCompressed()}
	return pk.Address().Bytes(), nil
}

func CreateChainKey(keyringDir, chainID, keyName, backend, passphrase, hdPath string) (*ChainKeyInfo, error) {
	sdkCtx, err := fpkr.CreateClientCtx(
		keyringDir, chainID,
	)
	if err != nil {
		return nil, err
	}

	krController, err := fpkr.NewChainKeyringController(
		sdkCtx,
		keyName,
		backend,
	)
	if err != nil {
		return nil, err
	}

	return krController.CreateChainKey(passphrase, hdPath)
}
