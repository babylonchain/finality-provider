package val

import (
	"fmt"

	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/go-bip39"
)

const (
	secp256k1Type       = "secp256k1"
	mnemonicEntropySize = 256
)

type ChainKeyringController struct {
	kr      keyring.Keyring
	valName string
}

func NewChainKeyringController(ctx client.Context, name, keyringBackend string) (*ChainKeyringController, error) {
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

	return &ChainKeyringController{
		valName: name,
		kr:      kr,
	}, nil
}

func NewChainKeyringControllerWithKeyring(kr keyring.Keyring, name string) (*ChainKeyringController, error) {
	if name == "" {
		return nil, fmt.Errorf("the key name should not be empty")
	}

	return &ChainKeyringController{
		kr:      kr,
		valName: name,
	}, nil
}

func (kc *ChainKeyringController) GetKeyring() keyring.Keyring {
	return kc.kr
}

func (kc *ChainKeyringController) CreateChainKey() (*secp256k1.PubKey, error) {
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
	fmt.Printf("Generated mnemonic for the validator %s is:\n%s\n", kc.valName, mnemonic)

	// TODO for now we leave bip39Passphrase and hdPath empty
	record, err := kc.kr.NewAccount(kc.valName, mnemonic, "", "", algo)
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
func (kc *ChainKeyringController) CreatePop(btcPrivKey *btcec.PrivateKey) (*bstypes.ProofOfPossession, error) {
	bbnPrivKey, err := kc.GetChainPrivKey()
	if err != nil {
		return nil, err
	}

	return bstypes.NewPoP(bbnPrivKey, btcPrivKey)
}

func (kc *ChainKeyringController) GetChainPrivKey() (*secp256k1.PrivKey, error) {
	k, err := kc.kr.Key(kc.valName)
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	privKeyCached := k.GetLocal().PrivKey.GetCachedValue()

	switch v := privKeyCached.(type) {
	case *secp256k1.PrivKey:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}
}
