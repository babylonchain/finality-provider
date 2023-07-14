package val

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"

	"github.com/babylonchain/btc-validator/valrpc"
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
	kr   keyring.Keyring
	name KeyName
}

func NewKeyringController(ctx client.Context, name string, keyringBackend string) (*KeyringController, error) {
	if name == "" {
		return nil, fmt.Errorf("the key name should not be empty")
	}
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

	return &KeyringController{
		name: KeyName(name),
		kr:   kr,
	}, nil
}

// CreateBTCValidator creates a BTC validator object using the keyring
func (kc *KeyringController) CreateBTCValidator() (*valrpc.Validator, error) {
	// create babylon key pair stored in the keyring
	babylonPubKey, err := kc.createBabylonKeyPair()
	if err != nil {
		return nil, err
	}

	// create BTC key pair stored in the keyring
	btcPubKey, err := kc.createBIP340KeyPair()
	if err != nil {
		return nil, err
	}

	// create proof of possession
	pop, err := kc.CreatePop()
	if err != nil {
		return nil, err
	}

	return NewValidator(babylonPubKey, btcPubKey, kc.GetKeyName(), pop), nil
}

func (kc *KeyringController) GetKeyName() string {
	return string(kc.name)
}

func (kc *KeyringController) KeyExists() bool {
	return kc.keyExists(kc.name.GetBabylonKeyName()) && kc.keyExists(kc.name.GetBtcKeyName())
}

func (kc *KeyringController) KeyNameTaken() bool {
	return kc.keyExists(kc.name.GetBabylonKeyName()) || kc.keyExists(kc.name.GetBtcKeyName())
}

func (kc *KeyringController) keyExists(name string) bool {
	_, err := kc.kr.Key(name)
	return err == nil
}

// createBabylonKeyPair creates a babylon key pair stored in the keyring
func (kc *KeyringController) createBabylonKeyPair() (*secp256k1.PubKey, error) {
	return kc.createKey(kc.name.GetBabylonKeyName())
}

// createBIP340KeyPair creates a BIP340 key pair stored in the keyring
func (kc *KeyringController) createBIP340KeyPair() (*types.BIP340PubKey, error) {
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
	// TODO use a better way to remind the user to keep it
	fmt.Printf("Generated mnemonic for key %s is %s\n", name, mnemonic)

	// TODO for now we leave bip39Passphrase and hdPath empty
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

// CreatePop creates proof-of-possession of Babylon and BTC public keys
// the input is the bytes of BTC public key used to sign
// this requires both keys created beforehand
func (kc *KeyringController) CreatePop() (*bstypes.ProofOfPossession, error) {
	if !kc.KeyExists() {
		return nil, fmt.Errorf("the keys do not exist")
	}

	bbnName := kc.name.GetBabylonKeyName()
	btcName := kc.name.GetBtcKeyName()

	// retrieve Babylon private key from keyring
	bbnPrivKey, _, err := kc.getKey(bbnName)
	if err != nil {
		return nil, err
	}

	// retrieve BTC private key from keyring
	btcPrivKey, _, err := kc.getKey(btcName)
	if err != nil {
		return nil, err
	}

	sdkPrivKey := &secp256k1.PrivKey{Key: bbnPrivKey.Serialize()}
	return bstypes.NewPoP(sdkPrivKey, btcPrivKey)
}

func (kc *KeyringController) getKey(name string) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	k, err := kc.kr.Key(name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get private key by name %s: %w", name, err)
	}

	privKey := k.GetLocal().PrivKey.GetCachedValue()

	var btcPrivKey *btcec.PrivateKey
	var btcPubKey *btcec.PublicKey
	switch v := privKey.(type) {
	case *secp256k1.PrivKey:
		btcPrivKey, btcPubKey = btcec.PrivKeyFromBytes(v.Key)
		return btcPrivKey, btcPubKey, nil
	default:
		return nil, nil, fmt.Errorf("unsupported key type in keyring")
	}
}
