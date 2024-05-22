package clientcontroller

import (
	"context"
	"fmt"
	"time"

	sdkErr "cosmossdk.io/errors"
	bbnclient "github.com/babylonchain/babylon/client/client"
	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ ConsumerController = &BabylonConsumerController{}

type BabylonConsumerController struct {
	bbnClient *bbnclient.Client
	cfg       *fpcfg.BBNConfig
	btcParams *chaincfg.Params
	logger    *zap.Logger
}

func NewBabylonConsumerController(
	cfg *fpcfg.BBNConfig,
	btcParams *chaincfg.Params,
	logger *zap.Logger,
) (*BabylonConsumerController, error) {

	bbnConfig := fpcfg.BBNConfigToBabylonConfig(cfg)

	if err := bbnConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon client: %w", err)
	}

	bc, err := bbnclient.New(
		&bbnConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	return &BabylonConsumerController{
		bc,
		cfg,
		btcParams,
		logger,
	}, nil
}

func (bc *BabylonConsumerController) mustGetTxSigner() string {
	signer := bc.GetKeyAddress()
	prefix := bc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (bc *BabylonConsumerController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonConsumerController, so if it fails something is really wrong

	keyRec, err := bc.bbnClient.GetKeyring().Key(bc.cfg.Key)

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	addr, err := keyRec.GetAddress()

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	return addr
}

func (bc *BabylonConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (bc *BabylonConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonConsumerController) SubmitFinalitySig(fpPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {
	msg := &finalitytypes.MsgAddFinalitySig{
		Signer:       bc.mustGetTxSigner(),
		FpBtcPk:      bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		BlockHeight:  blockHeight,
		BlockAppHash: blockHash,
		FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sig),
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidFinalitySig,
		finalitytypes.ErrPubRandNotFound,
		btcstakingtypes.ErrFpAlreadySlashed,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (bc *BabylonConsumerController) SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, b := range blocks {
		msg := &finalitytypes.MsgAddFinalitySig{
			Signer:       bc.mustGetTxSigner(),
			FpBtcPk:      bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
			BlockHeight:  b.Height,
			BlockAppHash: b.Hash,
			FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]),
		}
		msgs = append(msgs, msg)
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidFinalitySig,
		finalitytypes.ErrPubRandNotFound,
		btcstakingtypes.ErrFpAlreadySlashed,
	}

	res, err := bc.reliablySendMsgs(msgs, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (bc *BabylonConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProviderPowerAtHeight(
		bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
		blockHeight,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query the finality provider's voting power at height %d: %w", blockHeight, err)
	}

	return res.VotingPower, nil
}

func (bc *BabylonConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	blocks, err := bc.queryLatestBlocks(nil, 1, finalitytypes.QueriedBlockStatus_FINALIZED, true)
	if blocks == nil {
		return nil, err
	}
	return blocks[0], err
}

func (bc *BabylonConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	if endHeight < startHeight {
		return nil, fmt.Errorf("the startHeight %v should not be higher than the endHeight %v", startHeight, endHeight)
	}
	count := endHeight - startHeight + 1
	if count > limit {
		count = limit
	}
	return bc.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), count, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (bc *BabylonConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo
	pagination := &sdkquery.PageRequest{
		Limit:   count,
		Reverse: reverse,
		Key:     startKey,
	}

	res, err := bc.bbnClient.QueryClient.ListBlocks(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query finalized blocks: %v", err)
	}

	for _, b := range res.Blocks {
		ib := &types.BlockInfo{
			Height: b.Height,
			Hash:   b.AppHash,
		}
		blocks = append(blocks, ib)
	}

	return blocks, nil
}

func getContextWithCancel(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel
}

func (bc *BabylonConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	res, err := bc.bbnClient.QueryClient.Block(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed block at height %v: %w", height, err)
	}

	return &types.BlockInfo{
		Height: height,
		Hash:   res.Block.AppHash,
	}, nil
}

func (bc *BabylonConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	res, err := bc.bbnClient.QueryClient.Block(height)
	if err != nil {
		return false, fmt.Errorf("failed to query indexed block at height %v: %w", height, err)
	}

	return res.Block.Finalized, nil
}

func (bc *BabylonConsumerController) QueryActivatedHeight() (uint64, error) {
	res, err := bc.bbnClient.QueryClient.ActivatedHeight()
	if err != nil {
		return 0, fmt.Errorf("failed to query activated height: %w", err)
	}

	return res.Height, nil
}

func (bc *BabylonConsumerController) QueryLatestBlockHeight() (uint64, error) {
	blocks, err := bc.queryLatestBlocks(nil, 1, finalitytypes.QueriedBlockStatus_ANY, true)
	if err != nil || len(blocks) != 1 {
		// try query comet block if the index block query is not available
		block, err := bc.queryCometBestBlock()
		return block.Height, err
	}

	return blocks[0].Height, nil
}

func (bc *BabylonConsumerController) queryCometBestBlock() (*types.BlockInfo, error) {
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := bc.bbnClient.RPCClient.BlockchainInfo(ctx, 0, 0)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return &types.BlockInfo{
		Height: uint64(chainInfo.BlockMetas[0].Header.Height),
		Hash:   chainInfo.BlockMetas[0].Header.AppHash,
	}, nil
}

func (bc *BabylonConsumerController) Close() error {
	if !bc.bbnClient.IsRunning() {
		return nil
	}

	return bc.bbnClient.Stop()
}
