package eotsmanager

import (
	"fmt"
	"os"
	"path"

	"github.com/babylonchain/babylon/crypto/eots"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	secp256k12 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/codec"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/randgenerator"
	eotstypes "github.com/babylonchain/btc-validator/eotsmanager/types"
)

const (
	secp256k1Type       = "secp256k1"
	mnemonicEntropySize = 256
)

var _ EOTSManager = &LocalEOTSManager{}

type LocalEOTSManager struct {
	kr     keyring.Keyring
	es     *EOTSStore
	logger *logrus.Logger
}

func NewLocalEOTSManager(eotsCfg *config.Config, logger *logrus.Logger) (*LocalEOTSManager, error) {
	keyringDir := eotsCfg.KeyDirectory
	if keyringDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		keyringDir = path.Join(homeDir, ".eots-manager")
	}

	ctx := client.Context{}.
		WithCodec(codec.MakeCodec()).
		WithKeyringDir(keyringDir)

	if eotsCfg.KeyringBackend == "" {
		return nil, fmt.Errorf("the keyring backend should not be empty")
	}

	kr, err := keyring.New(
		ctx.ChainID,
		// TODO currently only support test backend
		eotsCfg.KeyringBackend,
		ctx.KeyringDir,
		ctx.Input,
		ctx.Codec,
		ctx.KeyringOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	es, err := NewEOTSStore(eotsCfg.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	return &LocalEOTSManager{
		kr:     kr,
		es:     es,
		logger: logger,
	}, nil
}

func (lm *LocalEOTSManager) CreateKey(name, passPhrase string) ([]byte, error) {
	if lm.keyExists(name) {
		return nil, eotstypes.ErrValidatorAlreadyExisted
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
	record, err := lm.kr.NewAccount(name, mnemonic, passPhrase, "", algo)
	if err != nil {
		return nil, err
	}

	pubKey, err := record.GetPubKey()
	if err != nil {
		return nil, err
	}

	var eotsPk *bbntypes.BIP340PubKey
	switch v := pubKey.(type) {
	case *secp256k1.PubKey:
		pk, err := btcec.ParsePubKey(v.Key)
		if err != nil {
			return nil, err
		}
		eotsPk = bbntypes.NewBIP340PubKeyFromBTCPK(pk)
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}

	if err := lm.es.saveValidatorKey(eotsPk.MustMarshal(), name); err != nil {
		return nil, err
	}

	lm.logger.Infof("successfully created an EOTS key %s: %s", name, eotsPk.MarshalHex())

	return eotsPk.MustMarshal(), nil
}

// TODO the current implementation is a PoC, which does not contain any anti-slasher mechanism
//
//	a simple anti-slasher mechanism could be that the manager remembers the tuple (valPk, chainID, height) or
//	the hash of each generated randomness and return error if the same randomness is requested tweice
func (lm *LocalEOTSManager) CreateRandomnessPairList(valPk []byte, chainID []byte, startHeight uint64, num uint32) ([]*btcec.FieldVal, error) {
	prList := make([]*btcec.FieldVal, 0, num)

	for i := uint32(0); i < num; i++ {
		height := startHeight + uint64(i)
		_, pubRand, err := lm.getRandomnessPair(valPk, chainID, height)
		if err != nil {
			return nil, err
		}

		prList = append(prList, pubRand)
	}

	return prList, nil
}

func (lm *LocalEOTSManager) SignEOTS(valPk []byte, chainID []byte, msg []byte, height uint64) (*btcec.ModNScalar, error) {
	privRand, _, err := lm.getRandomnessPair(valPk, chainID, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get private randomness: %w", err)
	}

	privKey, err := lm.getEOTSPrivKey(valPk)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOTS private key: %w", err)
	}

	return eots.Sign(privKey, privRand, msg)
}

func (lm *LocalEOTSManager) SignSchnorrSig(valPk []byte, msg []byte) (*schnorr.Signature, error) {
	privKey, err := lm.getEOTSPrivKey(valPk)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOTS private key: %w", err)
	}

	return schnorr.Sign(privKey, msg)
}

func (lm *LocalEOTSManager) Close() error {
	return lm.es.Close()
}

// getRandomnessPair returns a randomness pair generated based on the given validator key, chainID and height
func (lm *LocalEOTSManager) getRandomnessPair(valPk []byte, chainID []byte, height uint64) (*eots.PrivateRand, *secp256k12.FieldVal, error) {
	record, err := lm.KeyRecord(valPk, "")
	if err != nil {
		return nil, nil, err
	}
	privRand := randgenerator.GenerateRandomness(record.PrivKey.Serialize(), append(sdk.Uint64ToBigEndian(height), chainID...))
	var j secp256k12.JacobianPoint
	secp256k12.NewPrivateKey(privRand).PubKey().AsJacobian(&j)
	return privRand, &j.X, nil
}

// TODO: we ignore passPhrase in local implementation for now
func (lm *LocalEOTSManager) KeyRecord(valPk []byte, passPhrase string) (*eotstypes.KeyRecord, error) {
	name, err := lm.es.getValidatorKeyName(valPk)
	if err != nil {
		return nil, err
	}
	privKey, err := lm.getEOTSPrivKey(valPk)
	if err != nil {
		return nil, err
	}

	return &eotstypes.KeyRecord{
		Name:    name,
		PrivKey: privKey,
	}, nil
}

func (lm *LocalEOTSManager) getEOTSPrivKey(valPk []byte) (*btcec.PrivateKey, error) {
	keyName, err := lm.es.getValidatorKeyName(valPk)
	if err != nil {
		return nil, err
	}

	k, err := lm.kr.Key(keyName)
	if err != nil {
		return nil, err
	}

	privKeyCached := k.GetLocal().PrivKey.GetCachedValue()

	var privKey *btcec.PrivateKey
	switch v := privKeyCached.(type) {
	case *secp256k1.PrivKey:
		privKey, _ = btcec.PrivKeyFromBytes(v.Key)
		return privKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}
}

func (lm *LocalEOTSManager) keyExists(name string) bool {
	_, err := lm.kr.Key(name)
	return err == nil
}
