package eotsmanager

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/babylonchain/finality-provider/metrics"

	"github.com/babylonchain/babylon/crypto/eots"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"
	"github.com/lightningnetwork/lnd/kvdb"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/codec"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/eotsmanager/store"
	eotstypes "github.com/babylonchain/finality-provider/eotsmanager/types"
	fpkeyring "github.com/babylonchain/finality-provider/keyring"
)

const (
	secp256k1Type       = "secp256k1"
	MnemonicEntropySize = 256
)

var _ EOTSManager = &LocalEOTSManager{}

type LocalEOTSManager struct {
	kr     keyring.Keyring
	es     *store.EOTSStore
	logger *zap.Logger
	// input is to send passphrase to kr
	input   *strings.Reader
	metrics *metrics.EotsMetrics
}

func NewLocalEOTSManager(homeDir string, eotsCfg *config.Config, dbbackend kvdb.Backend, logger *zap.Logger) (*LocalEOTSManager, error) {
	inputReader := strings.NewReader("")

	es, err := store.NewEOTSStore(dbbackend)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	kr, err := initKeyring(homeDir, eotsCfg, inputReader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize keyring: %w", err)
	}

	eotsMetrics := metrics.NewEotsMetrics()

	return &LocalEOTSManager{
		kr:      kr,
		es:      es,
		logger:  logger,
		input:   inputReader,
		metrics: eotsMetrics,
	}, nil
}

func initKeyring(homeDir string, eotsCfg *config.Config, inputReader *strings.Reader) (keyring.Keyring, error) {
	return keyring.New(
		"eots-manager",
		eotsCfg.KeyringBackend,
		homeDir,
		inputReader,
		codec.MakeCodec(),
	)
}

func (lm *LocalEOTSManager) CreateKey(name, passphrase, hdPath string) ([]byte, error) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		return nil, err
	}

	eotsPk, err := lm.CreateKeyWithMnemonic(name, passphrase, hdPath, mnemonic)
	if err != nil {
		return nil, err
	}

	return eotsPk.MustMarshal(), nil
}

func NewMnemonic() (string, error) {
	// read entropy seed straight from tmcrypto.Rand and convert to mnemonic
	entropySeed, err := bip39.NewEntropy(MnemonicEntropySize)
	if err != nil {
		return "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

func (lm *LocalEOTSManager) CreateKeyWithMnemonic(name, passphrase, hdPath, mnemonic string) (*bbntypes.BIP340PubKey, error) {
	if lm.keyExists(name) {
		return nil, eotstypes.ErrFinalityProviderAlreadyExisted
	}

	keyringAlgos, _ := lm.kr.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(secp256k1Type, keyringAlgos)
	if err != nil {
		return nil, err
	}

	// we need to repeat the passphrase to mock the re-entry
	// as when creating an account, passphrase will be asked twice
	// by the keyring
	lm.input.Reset(passphrase + "\n" + passphrase)
	record, err := lm.kr.NewAccount(name, mnemonic, passphrase, hdPath, algo)
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

	if err := lm.es.AddEOTSKeyName(eotsPk.MustToBTCPK(), name); err != nil {
		return nil, err
	}

	lm.logger.Info(
		"successfully created an EOTS key",
		zap.String("key name", name),
		zap.String("pk", eotsPk.MarshalHex()),
	)
	lm.metrics.IncrementEotsCreatedKeysCounter()

	return eotsPk, nil
}

// CreateMasterRandPair creates a pair of master secret/public randomness deterministically
// from the finality provider's secret key and chain ID
func (lm *LocalEOTSManager) CreateMasterRandPair(fpPk []byte, chainID []byte, passphrase string) (string, error) {
	_, mpr, err := lm.getMasterRandPair(fpPk, chainID, passphrase)
	if err != nil {
		return "", err
	}

	return mpr.MarshalBase58(), nil
}

func (lm *LocalEOTSManager) SignEOTS(fpPk []byte, chainID []byte, msg []byte, height uint64, passphrase string) (*btcec.ModNScalar, error) {
	// get master secret randomness
	// TODO: instead of calculating master secret randomness everytime, is it possible
	// to manage it in the keyring?
	msr, _, err := lm.getMasterRandPair(fpPk, chainID, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get master secret randomness: %w", err)
	}

	// derive secret randomness
	sr, _, err := msr.DeriveRandPair(uint32(height)) // TODO: generalise to uint64
	if err != nil {
		return nil, fmt.Errorf("failed to get secret randomness: %w", err)
	}

	privKey, err := lm.getEOTSPrivKey(fpPk, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOTS private key: %w", err)
	}

	// Update metrics
	lm.metrics.IncrementEotsFpTotalEotsSignCounter(hex.EncodeToString(fpPk))
	lm.metrics.SetEotsFpLastEotsSignHeight(hex.EncodeToString(fpPk), float64(height))

	return eots.Sign(privKey, sr, msg)
}

func (lm *LocalEOTSManager) SignSchnorrSig(fpPk []byte, msg []byte, passphrase string) (*schnorr.Signature, error) {
	privKey, err := lm.getEOTSPrivKey(fpPk, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOTS private key: %w", err)
	}

	// Update metrics
	lm.metrics.IncrementEotsFpTotalSchnorrSignCounter(hex.EncodeToString(fpPk))

	return schnorr.Sign(privKey, msg)
}

func (lm *LocalEOTSManager) Close() error {
	return nil
}

// getMasterRandPair returns a randomness pair generated based on the given finality provider key, chainID and height
func (lm *LocalEOTSManager) getMasterRandPair(fpPk []byte, chainID []byte, passphrase string) (*eots.MasterSecretRand, *eots.MasterPublicRand, error) {
	record, err := lm.KeyRecord(fpPk, passphrase)
	if err != nil {
		return nil, nil, err
	}
	return fpkeyring.GenerateMasterRandPair(record.PrivKey.Serialize(), chainID)
}

// TODO: we ignore passPhrase in local implementation for now
func (lm *LocalEOTSManager) KeyRecord(fpPk []byte, passphrase string) (*eotstypes.KeyRecord, error) {
	name, err := lm.es.GetEOTSKeyName(fpPk)
	if err != nil {
		return nil, err
	}
	privKey, err := lm.getEOTSPrivKey(fpPk, passphrase)
	if err != nil {
		return nil, err
	}

	return &eotstypes.KeyRecord{
		Name:    name,
		PrivKey: privKey,
	}, nil
}

func (lm *LocalEOTSManager) getEOTSPrivKey(fpPk []byte, passphrase string) (*btcec.PrivateKey, error) {
	keyName, err := lm.es.GetEOTSKeyName(fpPk)
	if err != nil {
		return nil, err
	}

	lm.input.Reset(passphrase)
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
