package clientcontroller

import (
	"context"
	"encoding/json"
	"fmt"

	sdkErr "cosmossdk.io/errors"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbnclient "github.com/babylonchain/babylon/client/client"
	bbntypes "github.com/babylonchain/babylon/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ ConsumerController = &OPStackL2ConsumerController{}

type OPStackL2ConsumerController struct {
	bbnClient *bbnclient.Client
	cfg       *fpcfg.OPStackL2Config
	logger    *zap.Logger
}

func NewOPStackL2ConsumerController(
	bbnCfg *fpcfg.BBNConfig,
	opl2Cfg *fpcfg.OPStackL2Config,
	logger *zap.Logger,
) (*OPStackL2ConsumerController, error) {
	bbnConfig := fpcfg.BBNConfigToBabylonConfig(bbnCfg)
	if err := bbnConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon client: %w", err)
	}
	bbnClient, err := bbnclient.New(
		&bbnConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	keyRec, err := bbnClient.GetKeyring().Key(bbnConfig.Key)
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	// submitterAddress retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig.
	submitterAddress, err := keyRec.GetAddress()
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}
	bbnConfig.SubmitterAddress = submitterAddress.String()

	return &OPStackL2ConsumerController{
		bbnClient,
		opl2Cfg,
		logger,
	}, nil
}

func (ec *OPStackL2ConsumerController) ExecuteContract(contractAddress string, payload []byte) (*provider.RelayerTxResponse, error) {
	execMsg := &wasmtypes.MsgExecuteContract{
		Sender:   ec.bbnClient.MustGetAddr(),
		Contract: contractAddress,
		Msg:      payload,
	}

	res, err := ec.reliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (ec *OPStackL2ConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return ec.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (ec *OPStackL2ConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return ec.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// CommitPubRandList commits a list of Schnorr public randomness to Babylon CosmWasm contract
// it returns tx hash and error
func (ec *OPStackL2ConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	msg := types.CommitPublicRandomnessMsg{
		CommitPublicRandomness: types.CommitPublicRandomnessMsgParams{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig.Serialize(),
		},
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	res, err := ec.ExecuteContract(ec.cfg.OPFinalityGadgetAddress, msgData)
	if err != nil {
		return nil, err
	}
	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitFinalitySig submits the finality signature to Babylon CosmWasm contract
// it returns tx hash and error
func (ec *OPStackL2ConsumerController) SubmitFinalitySig(
	fpPk *btcec.PublicKey,
	block *types.BlockInfo,
	pubRand *btcec.FieldVal,
	proof []byte,
	sig *btcec.ModNScalar,
) (*types.TxResponse, error) {
	cmtProof := cmtcrypto.Proof{}
	if err := cmtProof.Unmarshal(proof); err != nil {
		return nil, err
	}

	msg := types.SubmitFinalitySignatureMsg{
		SubmitFinalitySignature: types.SubmitFinalitySignatureMsgParams{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			Height:      block.Height,
			PubRand:     bbntypes.NewSchnorrPubRandFromFieldVal(pubRand).MustMarshal(),
			Proof:       &cmtProof,
			BlockHash:   block.Hash,
			Signature:   bbntypes.NewSchnorrEOTSSigFromModNScalar(sig).MustMarshal(),
		},
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	res, err := ec.ExecuteContract(ec.cfg.OPFinalityGadgetAddress, msgData)
	if err != nil {
		return nil, err
	}
	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures
func (ec *OPStackL2ConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*types.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	return &types.TxResponse{TxHash: ""}, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (ec *OPStackL2ConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	/* TODO: implement

	latest_committed_l2_height = read `latestBlockNumber()` from the L1 L2OutputOracle contract and return the result

	if blockHeight > latest_committed_l2_height:

		query the VP from the L1 oracle contract using "latest" as the block tag

	else:

		1. query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
		  to find the first event where the `_l2BlockNumber` >= blockHeight
		2. get the block.number from the event
		3. query the VP from the L1 oracle contract using `block.number` as the block tag

	*/

	return 0, nil
}

func (ec *OPStackL2ConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	return &types.BlockInfo{
		Height: 0,
		Hash:   nil,
	}, nil
}

func (ec *OPStackL2ConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {

	return ec.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), 0, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (ec *OPStackL2ConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo

	return blocks, nil
}

func (ec *OPStackL2ConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {

	return &types.BlockInfo{
		Height: height,
		Hash:   nil,
	}, nil
}

func (ec *OPStackL2ConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	/* TODO: implement
	1. get the latest finalized block number from `latestBlockNumber()` in the L1 L2OutputOracle contract
	2. compare the block number with `height`
	*/
	return false, nil
}

func (ec *OPStackL2ConsumerController) QueryActivatedHeight() (uint64, error) {
	/* TODO: implement

		oracle_event = query the event in the L1 oracle contract where the FP's voting power is firstly set

		l1_activated_height = get the L1 block number from the `oracle_event`

	  output_event = query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
				to find the first event where the `block.number` >= l1_activated_height

		if output_event == nil:

				read `nextBlockNumber()` from the L1 L2OutputOracle contract and return the result

		else:

				return output_event._l2BlockNumber

	*/

	return 0, nil
}

func (ec *OPStackL2ConsumerController) QueryLatestBlockHeight() (uint64, error) {
	/* TODO: implement
	get the latest L2 block number from a RPC call
	*/

	return uint64(0), nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (ec *OPStackL2ConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {

	return nil, nil
}

func (ec *OPStackL2ConsumerController) Close() error {
	if err := ec.bbnClient.Stop(); err != nil {
		return err
	}
	return nil
}
