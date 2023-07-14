package babylonclient

import (
	"context"

	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bbncfg "github.com/babylonchain/rpc-client/config"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
	lensclient "github.com/strangelove-ventures/lens/client"
)

var _ BabylonClient = &BabylonController{}

type BabylonController struct {
	cc     *lensclient.ChainClient
	cfg    *bbncfg.BabylonConfig
	logger *logrus.Logger
}

func newLensClient(ccc *lensclient.ChainClientConfig, kro ...keyring.Option) (*lensclient.ChainClient, error) {
	// attach the supported algorithms to the keyring options
	keyringOptions := []keyring.Option{}
	keyringOptions = append(keyringOptions, func(options *keyring.Options) {
		options.SupportedAlgos = keyring.SigningAlgoList{hd.Secp256k1}
		options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Secp256k1}
	})
	keyringOptions = append(keyringOptions, kro...)

	cc := &lensclient.ChainClient{
		KeyringOptions: keyringOptions,
		Config:         ccc,
		Codec:          lensclient.MakeCodec(ccc.Modules, []string{}),
	}
	if err := cc.Init(); err != nil {
		return nil, err
	}

	if _, err := cc.GetKeyAddress(); err != nil {
		return nil, err
	}

	return cc, nil
}

func NewBabylonController(
	cfg *bbncfg.BabylonConfig,
	logger *logrus.Logger,
) (*BabylonController, error) {
	// create a Tendermint/Cosmos client for Babylon
	cc, err := newLensClient(cfg.Unwrap())
	if err != nil {
		return nil, err
	}

	return &BabylonController{
		cc,
		cfg,
		logger,
	}, nil
}

// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *btcstakingtypes.ProofOfPossession) ([]byte, error) {
	registerMsg := &btcstakingtypes.MsgCreateBTCValidator{
		BabylonPk: bbnPubKey,
		BtcPk:     btcPubKey,
		Pop:       pop,
	}

	res, err := bc.cc.SendMsg(context.Background(), registerMsg, "")
	if err != nil {
		return nil, err
	}

	return []byte(res.TxHash), nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error) {
	panic("implement me")
}

// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJurySig(btcPubKey *types.BIP340PubKey, delPubKey *types.BIP340PubKey, sig *types.BIP340Signature) ([]byte, error) {
	panic("implement me")
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) ([]byte, error) {
	panic("implement me")
}

// Note: the following queries are only for PoC
// QueryHeightWithLastPubRand queries the height of the last block with public randomness
func (bc *BabylonController) QueryHeightWithLastPubRand(btcPubKey *types.BIP340PubKey) (uint64, error) {
	panic("implement me")
}

// QueryShouldSubmitJurySigs queries if there's a list of delegations that the Jury should submit Jury sigs to
// it is only used when the program is running in Jury mode
// it returns a list of public keys used for delegations
func (bc *BabylonController) QueryShouldSubmitJurySigs(btcPubKey *types.BIP340PubKey) (bool, []*types.BIP340PubKey, error) {
	panic("implement me")
}

// QueryShouldValidatorVote asks Babylon if the validator should submit a finality sig for the given block height
func (bc *BabylonController) QueryShouldValidatorVote(btcPubKey *types.BIP340PubKey, blockHeight uint64) (bool, error) {
	panic("implement me")
}
