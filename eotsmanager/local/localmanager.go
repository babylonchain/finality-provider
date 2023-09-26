package local

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
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

func (lm *LocalEOTSManager) CreateValidator(name, passPhrase string) (*types.BIP340PubKey, error) {
	if lm.keyExists(name) {
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
	record, err := lm.kr.NewAccount(name, mnemonic, passPhrase, "", algo)
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
		KeyName: name,
	}

	if err := lm.es.saveValidator(validator); err != nil {
		return nil, err
	}

	return eotsPk, nil
}

func (lm *LocalEOTSManager) CreateRandomnessPairList(valPk *types.BIP340PubKey, chainID string, startHeight uint64, step int, num int) ([]*types.SchnorrPubRand, error) {
	prList := make([]*types.SchnorrPubRand, 0, num)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < num; i++ {
		// generate randomness pair
		eotsSR, eotsPR, err := eots.RandGen(r)
		if err != nil {
			return nil, fmt.Errorf("failed to generate randomness pair")
		}

		// persists randomness pair
		height := startHeight + uint64(i)
		privRand := eotsSR.Bytes()
		pubRand := eotsPR.Bytes()
		randPair := &proto.SchnorrRandPair{
			SecRand: privRand[:],
			PubRand: pubRand[:],
		}
		if err := lm.es.saveRandPair(valPk.MustMarshal(), chainID, height, randPair); err != nil {
			return nil, fmt.Errorf("failed to save randomness pair")
		}

		pr := types.NewSchnorrPubRandFromFieldVal(eotsPR)
		prList = append(prList, pr)
	}

	return prList, nil
}

func (lm *LocalEOTSManager) SignSchnorrSig(valPk *types.BIP340PubKey, msg []byte) (*schnorr.Signature, error) {
	keyName, err := lm.es.getValidatorKeyName(valPk.MustMarshal())
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
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}

	return schnorr.Sign(privKey, msg)
}

func (lm *LocalEOTSManager) keyExists(name string) bool {
	_, err := lm.kr.Key(name)
	return err == nil
}
