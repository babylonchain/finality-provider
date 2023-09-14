package val

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"

	"github.com/babylonchain/btc-validator/proto"
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

func NewKeyringControllerWithKeyring(kr keyring.Keyring, name string) (*KeyringController, error) {
	if name == "" {
		return nil, fmt.Errorf("the key name should not be empty")
	}
	return &KeyringController{
		kr:   kr,
		name: KeyName(name),
	}, nil
}

// CreateBTCValidator creates a BTC validator object using the keyring
func (kc *KeyringController) CreateBTCValidator(des string) (*proto.StoreValidator, error) {
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
	pop, err := kc.createPop()
	if err != nil {
		return nil, err
	}

	return NewStoreValidator(babylonPubKey, btcPubKey, kc.GetKeyName(), pop, des), nil
}

func (kc *KeyringController) GetKeyName() string {
	return string(kc.name)
}

func (kc *KeyringController) GetBabylonPublicKeyBytes() ([]byte, error) {
	k, err := kc.kr.Key(kc.name.GetBabylonKeyName())
	if err != nil {
		return nil, err
	}
	pubKey, err := k.GetPubKey()
	if err != nil {
		return nil, err
	}
	return pubKey.Bytes(), err
}

func (kc *KeyringController) ValidatorKeyExists() bool {
	return kc.keyExists(kc.name.GetBabylonKeyName()) && kc.keyExists(kc.name.GetBtcKeyName())
}

func (kc *KeyringController) ValidatorKeyNameTaken() bool {
	return kc.keyExists(kc.name.GetBabylonKeyName()) || kc.keyExists(kc.name.GetBtcKeyName())
}

func (kc *KeyringController) JuryKeyTaken() bool {
	return kc.keyExists(kc.GetKeyName())
}

func (kc *KeyringController) keyExists(name string) bool {
	_, err := kc.kr.Key(name)
	return err == nil
}

func (kc *KeyringController) GetKeyring() keyring.Keyring {
	return kc.kr
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

func (kc *KeyringController) CreateJuryKey() (*btcec.PublicKey, error) {
	sdkPk, err := kc.createKey(string(kc.name))
	if err != nil {
		return nil, err
	}

	return btcec.ParsePubKey(sdkPk.Key)
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

// createPop creates proof-of-possession of Babylon and BTC public keys
// the input is the bytes of BTC public key used to sign
// this requires both keys created beforehand
func (kc *KeyringController) createPop() (*bstypes.ProofOfPossession, error) {
	if !kc.ValidatorKeyExists() {
		return nil, fmt.Errorf("the keys do not exist")
	}

	btcPrivKey, err := kc.GetBtcPrivKey()
	if err != nil {
		return nil, err
	}

	bbnPrivKey, err := kc.GetBabylonPrivKey()
	if err != nil {
		return nil, err
	}
	return bstypes.NewPoP(bbnPrivKey, btcPrivKey)
}

func (kc *KeyringController) GetBabylonPrivKey() (*secp256k1.PrivKey, error) {
	bbnName := kc.name.GetBabylonKeyName()

	bbnPrivKey, _, err := kc.getKey(bbnName)
	if err != nil {
		return nil, err
	}

	sdkPrivKey := &secp256k1.PrivKey{Key: bbnPrivKey.Serialize()}

	return sdkPrivKey, nil
}

func (kc *KeyringController) GetBtcPrivKey() (*btcec.PrivateKey, error) {
	btcName := kc.name.GetBtcKeyName()

	btcPrivKey, _, err := kc.getKey(btcName)
	if err != nil {
		return nil, err
	}

	return btcPrivKey, nil
}

func (kc *KeyringController) getKey(name string) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	k, err := kc.kr.Key(name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get private key by name %s: %w", name, err)
	}

	privKeyCached := k.GetLocal().PrivKey.GetCachedValue()

	var privKey *btcec.PrivateKey
	var pubKey *btcec.PublicKey
	switch v := privKeyCached.(type) {
	case *secp256k1.PrivKey:
		privKey, pubKey = btcec.PrivKeyFromBytes(v.Key)
		return privKey, pubKey, nil
	default:
		return nil, nil, fmt.Errorf("unsupported key type in keyring")
	}
}

func (kc *KeyringController) SchnorrSign(msg []byte) (*schnorr.Signature, error) {
	btcPrivKey, _, err := kc.getKey(kc.name.GetBtcKeyName())
	if err != nil {
		return nil, err
	}

	return schnorr.Sign(btcPrivKey, msg)
}
