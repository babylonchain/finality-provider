package local

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	secp256k1Type       = "secp256k1"
	mnemonicEntropySize = 256
)

type LocalEOTSManager struct {
	kr keyring.Keyring
	es *EOTSStore
}

func NewLocalEOTSManager(ctx client.Context, keyringBackend string, dbCfg *valcfg.DatabaseConfig) (*LocalEOTSManager, error) {
	if keyringBackend == "" {
		return nil, fmt.Errorf("the keyring backend should not be empty")
	}

	kr, err := keyring.New(
		ctx.ChainID,
		keyringBackend,
		ctx.KeyringDir,
		ctx.Input,
		ctx.Codec,
		ctx.KeyringOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	es, err := NewEOTSStore(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	return &LocalEOTSManager{
		kr: kr,
		es: es,
	}, nil
}

func (lm *LocalEOTSManager) CreateValidator(uid, passPhrase string) (*types.BIP340PubKey, error) {
	if lm.keyExists(uid) {
		return nil, eotsmanager.ErrValidatorAlreadyExisted
	}

	keyringAlgos, _ := lm.kr.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(secp256k1Type, keyringAlgos)
	if err != nil {
		return nil, err
	}

	// read entropy seed straight from tmcrypto.Rand and convert to mnemonic
	entropySeed, err := bip39.NewEntropy(mnemonicEntropySize)
	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return nil, err
	}

	// TODO for now we leave and hdPath empty
	record, err := lm.kr.NewAccount(uid, mnemonic, passPhrase, "", algo)
	if err != nil {
		return nil, err
	}

	pubKey, err := record.GetPubKey()
	if err != nil {
		return nil, err
	}

	var eotsPk *types.BIP340PubKey
	switch v := pubKey.(type) {
	case *secp256k1.PubKey:
		pk, err := btcec.ParsePubKey(v.Key)
		if err != nil {
			return nil, err
		}
		eotsPk = types.NewBIP340PubKeyFromBTCPK(pk)
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}

	validator := &proto.StoreValidator{
		BtcPk:   eotsPk.MustMarshal(),
		KeyName: uid,
	}

	if err := lm.es.saveValidator(validator); err != nil {
		return nil, err
	}

	return eotsPk, nil
}

func (lm *LocalEOTSManager) keyExists(name string) bool {
	_, err := lm.kr.Key(name)
	return err == nil
}
