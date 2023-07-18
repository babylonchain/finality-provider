package babylonclient

import (
	"context"
	"fmt"

	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/rpc-client/client"
	bbncfg "github.com/babylonchain/rpc-client/config"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
)

var _ BabylonClient = &BabylonController{}

type BabylonController struct {
	rpcClient *client.Client
	logger    *logrus.Logger
}

func NewBabylonController(
	cfg *bbncfg.BabylonConfig,
	logger *logrus.Logger,
) (*BabylonController, error) {
	// create a Tendermint/Cosmos client for Babylon
	rpcClient, err := client.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create Babylon rpc client: %w", err)
	}

	return &BabylonController{
		rpcClient,
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

	res, err := bc.rpcClient.SendMsg(context.Background(), registerMsg, "")
	if err != nil {
		return nil, err
	}

	return []byte(res.TxHash), nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error) {
	msg := &finalitytypes.MsgCommitPubRandList{
		ValBtcPk:    btcPubKey,
		StartHeight: startHeight,
		PubRandList: pubRandList,
		Sig:         sig,
	}

	res, err := bc.rpcClient.SendMsg(context.Background(), msg, "")
	if err != nil {
		return nil, err
	}

	return []byte(res.TxHash), nil
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
