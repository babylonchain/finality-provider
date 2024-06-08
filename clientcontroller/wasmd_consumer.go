package clientcontroller

import (
	"context"
	"fmt"
	"os"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	cosmosclient "github.com/babylonchain/finality-provider/cosmoschainrpcclient/client"
	"github.com/babylonchain/finality-provider/cosmoschainrpcclient/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ ConsumerController = &BabylonConsumerController{}

type WasmdConsumerController struct {
	WasmdClient *cosmosclient.Client
	cfg         *config.CosmosChainConfig
	logger      *zap.Logger
}

func NewWasmdConsumerController(
	cfg *fpcfg.WasmdConfig,
	encodingConfig wasmdparams.EncodingConfig,
	logger *zap.Logger,
) (*WasmdConsumerController, error) {
	wasmdConfig := fpcfg.WasmdConfigToQueryClientConfig(cfg)

	if err := wasmdConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Wasmd client: %w", err)
	}

	wc, err := cosmosclient.New(
		wasmdConfig,
		"wasmd",
		encodingConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Wasmd client: %w", err)
	}

	return &WasmdConsumerController{
		wc,
		wasmdConfig,
		logger,
	}, nil
}

func (wc *WasmdConsumerController) mustGetTxSigner() string {
	signer := wc.GetKeyAddress()
	prefix := wc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (wc *WasmdConsumerController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonConsumerController, so if it fails something is really wrong

	keyRec, err := wc.WasmdClient.GetKeyring().Key(wc.cfg.Key)

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	addr, err := keyRec.GetAddress()

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	return addr
}

func (wc *WasmdConsumerController) ReliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (wc *WasmdConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.WasmdClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (wc *WasmdConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	// empty response
	return nil, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (wc *WasmdConsumerController) SubmitFinalitySig(
	fpPk *btcec.PublicKey,
	block *types.BlockInfo,
	pubRand *btcec.FieldVal,
	proof []byte, // TODO: have a type for proof
	sig *btcec.ModNScalar,
) (*types.TxResponse, error) {
	// empty response
	return nil, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (wc *WasmdConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*types.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*types.TxResponse, error) {
	// empty response
	return nil, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (wc *WasmdConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	// empty response
	return 0, nil
}

func (wc *WasmdConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	// empty response
	return nil, nil
}

func (wc *WasmdConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	// empty response
	return nil, nil
}

func (wc *WasmdConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	// empty response
	return nil, nil
}

func (wc *WasmdConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	// empty response
	return nil, nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (wc *WasmdConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {
	// empty response
	return nil, nil
}

func (wc *WasmdConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	// empty response
	return false, nil
}

func (wc *WasmdConsumerController) QueryActivatedHeight() (uint64, error) {
	// empty response
	return 0, nil
}

func (wc *WasmdConsumerController) QueryLatestBlockHeight() (uint64, error) {
	// empty response
	return 0, nil
}

func (wc *WasmdConsumerController) QueryCometBestBlock() (*types.BlockInfo, error) {
	ctx, cancel := getContextWithCancel(wc.cfg.Timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := wc.WasmdClient.RPCClient.BlockchainInfo(ctx, 0, 0)
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

//func (c *QueryClient) getQueryContext() (context.Context, context.CancelFunc) {
//	defaultOptions := DefaultQueryOptions()
//	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
//	strHeight := strconv.Itoa(int(defaultOptions.Height))
//	ctx = metadata.AppendToOutgoingContext(ctx, grpctypes.GRPCBlockHeightHeader, strHeight)
//	return ctx, cancel
//}
//
//func (wc *WasmdConsumerController) QueryLatestWasmCodeID() (*wasmtypes.CodeInfoResponse, error) {
//	ctx, cancel := wc.WasmdClient.getQueryContext()
//	defer cancel()
//
//	// Create a QueryClient
//	queryClient := wasmtypes.NewQueryClient(ctx)
//
//	// Create a QueryCodesRequest with pagination to limit to 1 item
//	pageReq := &query.PageRequest{
//		Limit:      1,
//		CountTotal: true,
//	}
//
//	// Query the codes
//	res, err := queryClient.Codes(
//		ctx,
//		&wasmtypes.QueryCodesRequest{
//			Pagination: pageReq,
//		},
//	)
//	if err != nil {
//		return nil, err
//	}
//
//	// Check if there are any codes
//	if len(res.CodeInfos) == 0 {
//		return nil, nil
//	}
//
//	// Return the latest code info (first item in the list)
//	return res.CodeInfos[0], nil
//}

func (wc *WasmdConsumerController) Close() error {
	if !wc.WasmdClient.IsRunning() {
		return nil
	}

	return wc.WasmdClient.Stop()
}

var tempDir = func() string {
	dir, err := os.MkdirTemp("", "wasmd")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	return dir
}
