package val

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"
)

const (
	btcPrefix           = "btc-"
	bbnPrefix           = "bbn-"
	secp256k1Type       = "secp256k1"
	mnemonicEntropySize = 256
)

type KeyName string

func (kn KeyName) GetBabylonKeyName() string {
	return bbnPrefix + string(kn)
}

func (kn KeyName) GetBtcKeyName() string {
	return btcPrefix + string(kn)
}

type KeyringController struct {
	name KeyName
	kr   keyring.Keyring
}

func NewKeyringController(name string, kr keyring.Keyring) *KeyringController {
	return &KeyringController{
		name: KeyName(name),
		kr:   kr,
	}
}

func (kc *KeyringController) GetKeyName() string {
	return string(kc.name)
}

func (kc *KeyringController) KeyExists() bool {
	return kc.keyExists(kc.name.GetBabylonKeyName()) || kc.keyExists(kc.name.GetBtcKeyName())
}

func (kc *KeyringController) keyExists(name string) bool {
	_, err := kc.kr.Key(name)
	return err == nil
}

func (kc *KeyringController) CreateBabylonKey() (*secp256k1.PubKey, error) {
	return kc.createKey(kc.name.GetBabylonKeyName())
}

func (kc *KeyringController) CreateBIP340PubKey() (*types.BIP340PubKey, error) {
	sdkPk, err := kc.createKey(kc.name.GetBtcKeyName())
	if err != nil {
		return nil, err
	}

	btcPk, err := btcec.ParsePubKey(sdkPk.Key)
	if err != nil {
		return nil, err
	}
	return types.NewBIP340PubKeyFromBTCPK(btcPk), nil
}

func (kc *KeyringController) createKey(name string) (*secp256k1.PubKey, error) {
	keyringAlgos, _ := kc.kr.SupportedAlgorithms()
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

	record, err := kc.kr.NewAccount(name, mnemonic, "", "", algo)
	if err != nil {
		return nil, err
	}

	pubKey, err := record.GetPubKey()
	if err != nil {
		return nil, err
	}

	switch v := pubKey.(type) {
	case *secp256k1.PubKey:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}
}

func (kc *KeyringController) CreatePop(btcPkBytes []byte) (*bstypes.ProofOfPossession, error) {
	bbnName := kc.name.GetBabylonKeyName()
	btcName := kc.name.GetBtcKeyName()

	fmt.Printf("trying to sign %x with key name %s\n", btcPkBytes, bbnName)
	bbnSig, _, err := kc.kr.Sign(bbnName, btcPkBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign with Babylon private key: %w", err)
	}

	k, err := kc.kr.Key(btcName)
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC key by name %s: %w", btcName, err)
	}

	privKey := k.GetLocal().PrivKey.GetCachedValue()

	var btcPrivKey *btcec.PrivateKey
	switch v := privKey.(type) {
	case *secp256k1.PrivKey:
		btcPrivKey, _ = btcec.PrivKeyFromBytes(v.Key)
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}

	schnorrSig, err := schnorr.Sign(btcPrivKey, bbnSig)
	if err != nil {
		return nil, fmt.Errorf("failed to sign with BTC private key: %w", err)
	}

	btcSig := types.NewBIP340SignatureFromBTCSig(schnorrSig)

	return &bstypes.ProofOfPossession{
		BabylonSig: bbnSig,
		BtcSig:     &btcSig,
	}, nil
}
