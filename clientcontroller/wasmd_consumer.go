package clientcontroller

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"strings"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	wasmclient "github.com/babylonchain/finality-provider/wasmclient/client"
	"github.com/babylonchain/finality-provider/wasmclient/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ ConsumerController = &WasmdConsumerController{}

type WasmdConsumerController struct {
	WasmdClient *wasmclient.Client
	cfg         *config.WasmConfig
	logger      *zap.Logger
}

func NewWasmdConsumerController(
	cfg *fpcfg.WasmConfig,
	encodingConfig wasmdparams.EncodingConfig,
	logger *zap.Logger,
) (*WasmdConsumerController, error) {
	wasmdConfig := fpcfg.ToQueryClientConfig(cfg)

	if err := wasmdConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Wasmd client: %w", err)
	}

	wc, err := wasmclient.New(
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

//nolint:unused
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

func (wc *WasmdConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
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

//nolint:unused
func (wc *WasmdConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	// TODO: not used right now, will be used to return latest indexed blocks once implemented in the smart contract
	return nil, nil
}

func (wc *WasmdConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	// TODO: dummy response, fetch actual block from the smart contract
	block, err := wc.queryCometBestBlock()
	if err != nil {
		return nil, err
	}

	return &types.BlockInfo{
		Height: height,
		Hash:   block.Hash,
	}, nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (wc *WasmdConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {
	// empty response
	return nil, nil
}

func (wc *WasmdConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	// TODO: dummy response, fetch actual finalized block from the smart contract
	return true, nil
}

func (wc *WasmdConsumerController) QueryActivatedHeight() (uint64, error) {
	// TODO: dummy response, fetch actual activated height from the smart contract
	return 1, nil
}

func (wc *WasmdConsumerController) QueryLatestBlockHeight() (uint64, error) {
	// TODO: dummy response, fetch actual latest indexed block from the smart contract
	block, err := wc.queryCometBestBlock()
	if err != nil {
		return 0, err
	}

	return block.Height, nil
}

func (wc *WasmdConsumerController) queryCometBestBlock() (*types.BlockInfo, error) {
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

func (wc *WasmdConsumerController) Exec(contract sdk.AccAddress, payload []byte) error {
	execMsg := &wasmdtypes.MsgExecuteContract{
		Sender:   wc.WasmdClient.MustGetAddr(),
		Contract: contract.String(),
		Msg:      payload,
	}

	_, err := wc.reliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// QuerySmartContractState queries the smart contract state
// NOTE: this function is only meant to be used in tests.
func (wc *WasmdConsumerController) QuerySmartContractState(contractAddress string, queryData string) (*wasmdtypes.QuerySmartContractStateResponse, error) {
	return wc.WasmdClient.QuerySmartContractState(contractAddress, queryData)
}

// StoreWasmCode stores the wasm code on the consumer chain
// NOTE: this function is only meant to be used in tests.
func (wc *WasmdConsumerController) StoreWasmCode(wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmdtypes.MsgStoreCode{
		Sender:       wc.WasmdClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = wc.reliablySendMsg(storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// InstantiateContract instantiates a contract with the given code id and init msg
// NOTE: this function is only meant to be used in tests.
func (wc *WasmdConsumerController) InstantiateContract(codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: wc.WasmdClient.MustGetAddr(),
		Admin:  wc.WasmdClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "ibc-test",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := wc.reliablySendMsg(instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// GetLatestCodeId returns the latest wasm code id.
// NOTE: this function is only meant to be used in tests.
func (wc *WasmdConsumerController) GetLatestCodeId() (uint64, error) {
	pagination := &sdkquerytypes.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := wc.WasmdClient.ListCodes(pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}

// ListContractsByCode lists all contracts by wasm code id
// NOTE: this function is only meant to be used in tests.
func (wc *WasmdConsumerController) ListContractsByCode(codeID uint64, pagination *sdkquerytypes.PageRequest) (*wasmdtypes.QueryContractsByCodeResponse, error) {
	return wc.WasmdClient.ListContractsByCode(codeID, pagination)
}
