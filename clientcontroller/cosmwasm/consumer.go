package cosmwasm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/clientcontroller/api"
	cwcclient "github.com/babylonchain/finality-provider/cosmwasmclient/client"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	fptypes "github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ api.ConsumerController = &CosmwasmConsumerController{}

type CosmwasmConsumerController struct {
	cwClient *cwcclient.Client
	cfg      *fpcfg.CosmwasmConfig
	logger   *zap.Logger
}

func NewCosmwasmConsumerController(
	cfg *fpcfg.CosmwasmConfig,
	encodingCfg wasmdparams.EncodingConfig,
	logger *zap.Logger,
) (*CosmwasmConsumerController, error) {
	wasmdConfig := cfg.ToQueryClientConfig()

	if err := wasmdConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Wasmd client: %w", err)
	}

	wc, err := cwcclient.New(
		wasmdConfig,
		"wasmd",
		encodingCfg,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Wasmd client: %w", err)
	}

	return &CosmwasmConsumerController{
		wc,
		cfg,
		logger,
	}, nil
}

func (wc *CosmwasmConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (wc *CosmwasmConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.cwClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (wc *CosmwasmConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*fptypes.TxResponse, error) {
	bip340Sig := bbntypes.NewBIP340SignatureFromBTCSig(sig).MustMarshal()

	// Construct the ExecMsg struct
	msg := ExecMsg{
		CommitPublicRandomness: &CommitPublicRandomness{
			FPPubKeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  base64.StdEncoding.EncodeToString(commitment),
			Signature:   base64.StdEncoding.EncodeToString(bip340Sig),
		},
	}

	// Marshal the ExecMsg struct to JSON
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	res, err := wc.ExecuteContract(msgBytes)
	if err != nil {
		return nil, err
	}

	return &fptypes.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (wc *CosmwasmConsumerController) SubmitFinalitySig(
	fpPk *btcec.PublicKey,
	block *fptypes.BlockInfo,
	pubRand *btcec.FieldVal,
	proof []byte, // TODO: have a type for proof
	sig *btcec.ModNScalar,
) (*fptypes.TxResponse, error) {
	cmtProof := cmtcrypto.Proof{}
	if err := cmtProof.Unmarshal(proof); err != nil {
		return nil, err
	}

	msg := ExecMsg{
		SubmitFinalitySignature: &SubmitFinalitySignature{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			Height:      block.Height,
			PubRand:     base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrPubRandFromFieldVal(pubRand).MustMarshal()),
			Proof:       ConvertProof(cmtProof),
			BlockHash:   base64.StdEncoding.EncodeToString(block.Hash),
			Signature:   base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrEOTSSigFromModNScalar(sig).MustMarshal()),
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	res, err := wc.ExecuteContract(msgBytes)
	if err != nil {
		return nil, err
	}

	return &fptypes.TxResponse{TxHash: res.TxHash, Events: fromCosmosEventsToBytes(res.Events)}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (wc *CosmwasmConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*fptypes.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*fptypes.TxResponse, error) {
	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, b := range blocks {
		cmtProof := cmtcrypto.Proof{}
		if err := cmtProof.Unmarshal(proofList[i]); err != nil {
			return nil, err
		}

		msg := ExecMsg{
			SubmitFinalitySignature: &SubmitFinalitySignature{
				FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
				Height:      b.Height,
				PubRand:     base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrPubRandFromFieldVal(pubRandList[i]).MustMarshal()),
				Proof:       ConvertProof(cmtProof),
				BlockHash:   base64.StdEncoding.EncodeToString(b.Hash),
				Signature:   base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]).MustMarshal()),
			},
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}

		execMsg := &wasmdtypes.MsgExecuteContract{
			Sender:   wc.cwClient.MustGetAddr(),
			Contract: sdk.MustAccAddressFromBech32(wc.cfg.BtcStakingContractAddress).String(),
			Msg:      msgBytes,
		}
		msgs = append(msgs, execMsg)
	}

	res, err := wc.reliablySendMsgs(msgs, nil, nil)
	if err != nil {
		return nil, err
	}

	return &fptypes.TxResponse{TxHash: res.TxHash}, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (wc *CosmwasmConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	fpBtcPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex()

	queryMsgStruct := QueryMsgFinalityProviderInfo{
		FinalityProviderInfo: FinalityProviderInfo{
			BtcPkHex: fpBtcPkHex,
			Height:   blockHeight,
		},
	}
	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query message: %v", err)
	}
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return 0, err
	}

	var resp ConsumerFpInfoResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return 0, err
	}

	return resp.Power, nil
}

func (wc *CosmwasmConsumerController) QueryFinalityProvidersByPower() (*ConsumerFpsByPowerResponse, error) {
	queryMsgStruct := QueryMsgFinalityProvidersByPower{
		FinalityProvidersByPower: struct{}{},
	}

	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, err
	}

	var resp ConsumerFpsByPowerResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *CosmwasmConsumerController) QueryLatestFinalizedBlock() (*fptypes.BlockInfo, error) {
	//isFinalized := true
	//limit := uint64(1)
	//blocks, err := wc.queryLatestBlocks(nil, &limit, &isFinalized, nil)
	//if err != nil {
	//	return nil, err
	//}
	//if len(blocks) == 0 {
	//	return nil, fmt.Errorf("no finalized blocks found")
	//}
	//
	//return blocks[0], nil

	// TODO: temporary hack get the block from comet
	latestHeight, err := wc.QueryLatestBlockHeight()
	if err != nil {
		return nil, err
	}
	qHeight := latestHeight / 3
	if qHeight == 0 {
		qHeight = 1
	}
	block, err := wc.QueryBlock(qHeight)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (wc *CosmwasmConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*fptypes.BlockInfo, error) {
	//if endHeight < startHeight {
	//	return nil, fmt.Errorf("the startHeight %v should not be higher than the endHeight %v", startHeight, endHeight)
	//}
	//count := endHeight - startHeight + 1
	//if count > limit {
	//	count = limit
	//}
	//
	//return wc.queryLatestBlocks(&startHeight, &count, nil, nil)

	// TODO: temporary hack get the block from comet
	return wc.queryCometBlocksInRange(startHeight, endHeight)
}

//nolint:unused
func (wc *CosmwasmConsumerController) queryLatestBlocks(startAfter, limit *uint64, finalized, reverse *bool) ([]*fptypes.BlockInfo, error) {
	// Construct the query message
	queryMsg := QueryMsgBlocks{
		Blocks: BlocksQuery{
			StartAfter: startAfter,
			Limit:      limit,
		},
	}

	// Marshal the query message to JSON
	queryMsgBytes, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %w", err)
	}

	// Query the smart contract state
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// Unmarshal the response
	var resp BlocksResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Process the blocks and convert them to BlockInfo
	var blocks []*fptypes.BlockInfo
	for _, b := range resp.Blocks {
		block := &fptypes.BlockInfo{
			Height: b.Height,
			Hash:   b.AppHash,
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

//nolint:unused
func (wc *CosmwasmConsumerController) queryIndexedBlock(height uint64) (*IndexedBlock, error) {
	// Construct the query message
	queryMsgStruct := QueryMsgBlock{
		Block: BlockQuery{
			Height: height,
		},
	}
	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	// Query the smart contract state
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// Unmarshal the response
	var resp IndexedBlock
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func (wc *CosmwasmConsumerController) QueryBlock(height uint64) (*fptypes.BlockInfo, error) {
	//// Use the helper function to get the IndexedBlock
	//resp, err := wc.queryIndexedBlock(height)
	//if err != nil {
	//	return nil, err
	//}
	//// Convert to BlockInfo and return
	//return &fptypes.BlockInfo{
	//	Height: resp.Height,
	//	Hash:   resp.AppHash,
	//}, nil

	// TODO: temporary hack get the block from comet
	block, err := wc.cwClient.GetBlock(int64(height))
	if err != nil {
		return nil, err
	}
	return &fptypes.BlockInfo{
		Height: uint64(block.Block.Header.Height),
		Hash:   block.Block.Header.AppHash,
	}, nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (wc *CosmwasmConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*fptypes.PubRandCommit, error) {
	fpBtcPk := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)

	// Construct the query message
	queryMsgStruct := QueryMsgLastPubRandCommit{
		LastPubRandCommit: LastPubRandCommitQuery{
			BtcPkHex: fpBtcPk.MarshalHex(),
			Limit:    &count,
		},
	}

	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	// Query the smart contract state
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// Define a response struct
	var commits []PubRandCommitResponse
	err = json.Unmarshal(dataFromContract.Data, &commits)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert the response to the expected map format
	commitMap := make(map[uint64]*fptypes.PubRandCommit)
	for _, commit := range commits {
		commitCopy := commit // create a copy to avoid referencing the loop variable
		commitMap[commit.StartHeight] = &fptypes.PubRandCommit{
			NumPubRand: commitCopy.NumPubRand,
			Commitment: commitCopy.Commitment,
		}
	}

	return commitMap, nil
}

func (wc *CosmwasmConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	//// Use the helper function to get the IndexedBlock
	//resp, err := wc.queryIndexedBlock(height)
	//if err != nil {
	//	return false, err
	//}
	//
	//// Return the finalized status
	//return resp.Finalized, nil

	// TODO: temporary hack get the block from comet
	_, err := wc.queryCometBestBlock()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (wc *CosmwasmConsumerController) QueryActivatedHeight() (uint64, error) {
	// Construct the query message
	queryMsg := QueryMsgActivatedHeight{
		ActivatedHeight: struct{}{},
	}

	// Marshal the query message to JSON
	queryMsgBytes, err := json.Marshal(queryMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query message: %w", err)
	}

	// Query the smart contract state
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return 0, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// Unmarshal the response
	var resp struct {
		Height uint64 `json:"height"`
	}
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Return the activated height
	return resp.Height, nil
}

func (wc *CosmwasmConsumerController) QueryLatestBlockHeight() (uint64, error) {
	//reverse := true
	//count := uint64(1)
	//blocks, err := wc.queryLatestBlocks(nil, &count, nil, &reverse)
	//if err != nil {
	//	return 0, err
	//}
	//
	//if len(blocks) == 0 {
	//	return 0, fmt.Errorf("no blocks found")
	//}
	//
	//return blocks[0].Height, nil

	// TODO: temporary hack get the block from comet
	block, err := wc.queryCometBestBlock()
	if err != nil {
		return 0, err
	}
	return block.Height, err
}

func (wc *CosmwasmConsumerController) QueryFinalitySignature(fpBtcPkHex string, height uint64) (*FinalitySignatureResponse, error) {
	queryMsgStruct := QueryMsgFinalitySignature{
		FinalitySignature: FinalitySignatureQuery{
			BtcPkHex: fpBtcPkHex,
			Height:   height,
		},
	}
	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, err
	}

	var resp FinalitySignatureResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *CosmwasmConsumerController) QueryFinalityProviders() (*ConsumerFpsResponse, error) {
	queryMsgStruct := QueryMsgFinalityProviders{
		FinalityProviders: struct{}{},
	}

	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, err
	}

	var resp ConsumerFpsResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *CosmwasmConsumerController) QueryDelegations() (*ConsumerDelegationsResponse, error) {
	queryMsgStruct := QueryMsgDelegations{
		Delegations: struct{}{},
	}

	queryMsgBytes, err := json.Marshal(queryMsgStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query message: %v", err)
	}

	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, string(queryMsgBytes))
	if err != nil {
		return nil, err
	}

	var resp ConsumerDelegationsResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (wc *CosmwasmConsumerController) queryCometBestBlock() (*fptypes.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wc.cfg.Timeout)
	defer cancel()

	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := wc.cwClient.RPCClient.BlockchainInfo(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return &fptypes.BlockInfo{
		Height: uint64(chainInfo.BlockMetas[0].Header.Height),
		Hash:   chainInfo.BlockMetas[0].Header.AppHash,
	}, nil
}

func (wc *CosmwasmConsumerController) queryCometBlocksInRange(startHeight, endHeight uint64) ([]*fptypes.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wc.cfg.Timeout)
	defer cancel()

	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := wc.cwClient.RPCClient.BlockchainInfo(ctx, int64(startHeight), int64(endHeight))
	if err != nil {
		return nil, err
	}

	// If no blocks found, return an empty slice
	if len(chainInfo.BlockMetas) == 0 {
		return nil, fmt.Errorf("no comet blocks found in the range")
	}

	// Process the blocks and convert them to BlockInfo
	var blocks []*fptypes.BlockInfo
	for _, blockMeta := range chainInfo.BlockMetas {
		block := &fptypes.BlockInfo{
			Height: uint64(blockMeta.Header.Height),
			Hash:   blockMeta.Header.AppHash,
		}
		blocks = append(blocks, block)
	}

	// Sort the blocks by height in ascending order
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Height < blocks[j].Height
	})

	return blocks, nil
}

func (wc *CosmwasmConsumerController) Close() error {
	if !wc.cwClient.IsRunning() {
		return nil
	}

	return wc.cwClient.Stop()
}

func (wc *CosmwasmConsumerController) ExecuteContract(msgBytes []byte) (*provider.RelayerTxResponse, error) {
	execMsg := &wasmdtypes.MsgExecuteContract{
		Sender:   wc.cwClient.MustGetAddr(),
		Contract: wc.cfg.BtcStakingContractAddress,
		Msg:      msgBytes,
	}

	res, err := wc.reliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QuerySmartContractState queries the smart contract state
// NOTE: this function is only meant to be used in tests.
func (wc *CosmwasmConsumerController) QuerySmartContractState(contractAddress string, queryData string) (*wasmdtypes.QuerySmartContractStateResponse, error) {
	return wc.cwClient.QuerySmartContractState(contractAddress, queryData)
}

// StoreWasmCode stores the wasm code on the consumer chain
// NOTE: this function is only meant to be used in tests.
func (wc *CosmwasmConsumerController) StoreWasmCode(wasmFile string) error {
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
		Sender:       wc.cwClient.MustGetAddr(),
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
func (wc *CosmwasmConsumerController) InstantiateContract(codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: wc.cwClient.MustGetAddr(),
		Admin:  wc.cwClient.MustGetAddr(),
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
func (wc *CosmwasmConsumerController) GetLatestCodeId() (uint64, error) {
	pagination := &sdkquerytypes.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := wc.cwClient.ListCodes(pagination)
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
func (wc *CosmwasmConsumerController) ListContractsByCode(codeID uint64, pagination *sdkquerytypes.PageRequest) (*wasmdtypes.QueryContractsByCodeResponse, error) {
	return wc.cwClient.ListContractsByCode(codeID, pagination)
}

// SetBtcStakingContractAddress updates the BtcStakingContractAddress in the configuration
// NOTE: this function is only meant to be used in tests.
func (wc *CosmwasmConsumerController) SetBtcStakingContractAddress(newAddress string) {
	wc.cfg.BtcStakingContractAddress = newAddress
}

// MustGetValidatorAddress gets the validator address of the consumer chain
// NOTE: this function is only meant to be used in tests.
func (wc *CosmwasmConsumerController) MustGetValidatorAddress() string {
	return wc.cwClient.MustGetAddr()
}

// GetCometNodeStatus gets the tendermint node status of the consumer chain
// NOTE: this function is only meant to be used in tests.
func (wc *CosmwasmConsumerController) GetCometNodeStatus() (*coretypes.ResultStatus, error) {
	return wc.cwClient.GetStatus()
}

func fromCosmosEventsToBytes(events []provider.RelayerEvent) []byte {
	bytes, err := json.Marshal(events)
	if err != nil {
		return nil
	}
	return bytes
}

func ConvertProof(cmtProof cmtcrypto.Proof) Proof {
	var aunts []string
	for _, aunt := range cmtProof.Aunts {
		aunts = append(aunts, base64.StdEncoding.EncodeToString(aunt))
	}

	return Proof{
		Total:    uint64(cmtProof.Total),
		Index:    uint64(cmtProof.Index),
		LeafHash: base64.StdEncoding.EncodeToString(cmtProof.LeafHash),
		Aunts:    aunts,
	}
}
