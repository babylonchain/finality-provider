package clientcontroller

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbn "github.com/babylonchain/babylon/types"
	bbntypes "github.com/babylonchain/babylon/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	cosmwasmclient "github.com/babylonchain/finality-provider/cosmwasmclient/client"
	"github.com/babylonchain/finality-provider/cosmwasmclient/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	fptypes "github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ ConsumerController = &CosmwasmConsumerController{}

type CosmwasmConsumerController struct {
	CosmwasmClient *cosmwasmclient.Client
	cfg            *config.CosmwasmConfig
	logger         *zap.Logger
}

func NewCosmwasmConsumerController(
	cfg *fpcfg.CosmwasmConfig,
	encodingCfg wasmdparams.EncodingConfig,
	logger *zap.Logger,
) (*CosmwasmConsumerController, error) {
	wasmdConfig := fpcfg.ToQueryClientConfig(cfg)

	if err := wasmdConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Wasmd client: %w", err)
	}

	wc, err := cosmwasmclient.New(
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
		wasmdConfig,
		logger,
	}, nil
}

//nolint:unused
func (wc *CosmwasmConsumerController) mustGetTxSigner() string {
	signer := wc.GetKeyAddress()
	prefix := wc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (wc *CosmwasmConsumerController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonConsumerController, so if it fails something is really wrong

	keyRec, err := wc.CosmwasmClient.GetKeyring().Key(wc.cfg.Key)

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	addr, err := keyRec.GetAddress()

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	return addr
}

func (wc *CosmwasmConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (wc *CosmwasmConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return wc.CosmwasmClient.ReliablySendMsgs(
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
	msg := fptypes.PubRandomnessExecMsg{
		CommitPublicRandomness: fptypes.CommitPublicRandomness{
			FPPubKeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  base64.StdEncoding.EncodeToString(commitment),
			Signature:   base64.StdEncoding.EncodeToString(bip340Sig),
		},
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err

	}

	res, err := wc.Exec(sdk.MustAccAddressFromBech32(wc.cfg.BtcStakingContractAddress), msgBytes)
	if err != nil {
		return nil, err
	}

	return &fptypes.TxResponse{TxHash: res.TxHash, Events: fromCosmosEventsToBytes(res.Events)}, nil
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

	var aunts []string
	for _, aunt := range cmtProof.Aunts {
		aunts = append(aunts, base64.StdEncoding.EncodeToString(aunt))
	}

	msg := fptypes.FinalitySigExecMsg{
		SubmitFinalitySignature: fptypes.SubmitFinalitySignature{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			Height:      block.Height,
			PubRand:     base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrPubRandFromFieldVal(pubRand).MustMarshal()),
			Proof: fptypes.Proof{
				Total:    uint64(cmtProof.Total),
				Index:    uint64(cmtProof.Index),
				LeafHash: base64.StdEncoding.EncodeToString(cmtProof.LeafHash),
				Aunts:    aunts,
			},
			BlockHash: base64.StdEncoding.EncodeToString(block.Hash),
			Signature: base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrEOTSSigFromModNScalar(sig).MustMarshal()),
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err

	}

	res, err := wc.Exec(sdk.MustAccAddressFromBech32(wc.cfg.BtcStakingContractAddress), msgBytes)
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

		var aunts []string
		for _, aunt := range cmtProof.Aunts {
			aunts = append(aunts, base64.StdEncoding.EncodeToString(aunt))
		}

		payload := fptypes.FinalitySigExecMsg{
			SubmitFinalitySignature: fptypes.SubmitFinalitySignature{
				FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
				Height:      b.Height,
				PubRand:     base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrPubRandFromFieldVal(pubRandList[i]).MustMarshal()),
				Proof: fptypes.Proof{
					Total:    uint64(cmtProof.Total),
					Index:    uint64(cmtProof.Index),
					LeafHash: base64.StdEncoding.EncodeToString(cmtProof.LeafHash),
					Aunts:    aunts,
				},
				BlockHash: base64.StdEncoding.EncodeToString(b.Hash),
				Signature: base64.StdEncoding.EncodeToString(bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]).MustMarshal()),
			},
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err

		}

		execMsg := &wasmdtypes.MsgExecuteContract{
			Sender:   wc.CosmwasmClient.MustGetAddr(),
			Contract: sdk.MustAccAddressFromBech32(wc.cfg.BtcStakingContractAddress).String(),
			Msg:      payloadBytes,
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
	fpBtcPkHex := bbn.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex()
	queryMsg := fmt.Sprintf(`{"finality_provider_info":{"btc_pk_hex":"%s", "height": %d}}`, fpBtcPkHex, blockHeight)
	dataFromContract, err := wc.QuerySmartContractState(wc.cfg.BtcStakingContractAddress, queryMsg)
	if err != nil {
		return 0, err
	}

	var resp fptypes.SingleConsumerFpPowerResponse
	err = json.Unmarshal(dataFromContract.Data, &resp)
	if err != nil {
		return 0, err
	}

	return resp.Power, nil
}

func (wc *CosmwasmConsumerController) QueryLatestFinalizedBlock() (*fptypes.BlockInfo, error) {
	// empty response
	return nil, nil
}

func (wc *CosmwasmConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*fptypes.BlockInfo, error) {
	// empty response
	return nil, nil
}

//nolint:unused
func (wc *CosmwasmConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*fptypes.BlockInfo, error) {
	// TODO: not used right now, will be used to return latest indexed blocks once implemented in the smart contract
	return nil, nil
}

func (wc *CosmwasmConsumerController) QueryBlock(height uint64) (*fptypes.BlockInfo, error) {
	// TODO: dummy response, fetch actual block from the smart contract
	block, err := wc.queryCometBestBlock()
	if err != nil {
		return nil, err
	}

	return &fptypes.BlockInfo{
		Height: height,
		Hash:   block.Hash,
	}, nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (wc *CosmwasmConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {
	// empty response
	return nil, nil
}

func (wc *CosmwasmConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	// TODO: dummy response, fetch actual finalized block from the smart contract
	return true, nil
}

func (wc *CosmwasmConsumerController) QueryActivatedHeight() (uint64, error) {
	// TODO: dummy response, fetch actual activated height from the smart contract
	return 1, nil
}

func (wc *CosmwasmConsumerController) QueryLatestBlockHeight() (uint64, error) {
	// TODO: dummy response, fetch actual latest indexed block from the smart contract
	block, err := wc.queryCometBestBlock()
	if err != nil {
		return 0, err
	}

	return block.Height, nil
}

func (wc *CosmwasmConsumerController) queryCometBestBlock() (*fptypes.BlockInfo, error) {
	ctx, cancel := getContextWithCancel(wc.cfg.Timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := wc.CosmwasmClient.RPCClient.BlockchainInfo(ctx, 0, 0)
	defer cancel()

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

func (wc *CosmwasmConsumerController) Close() error {
	if !wc.CosmwasmClient.IsRunning() {
		return nil
	}

	return wc.CosmwasmClient.Stop()
}

func (wc *CosmwasmConsumerController) Exec(contract sdk.AccAddress, payload []byte) (*provider.RelayerTxResponse, error) {
	execMsg := &wasmdtypes.MsgExecuteContract{
		Sender:   wc.CosmwasmClient.MustGetAddr(),
		Contract: contract.String(),
		Msg:      payload,
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
	return wc.CosmwasmClient.QuerySmartContractState(contractAddress, queryData)
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
		Sender:       wc.CosmwasmClient.MustGetAddr(),
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
		Sender: wc.CosmwasmClient.MustGetAddr(),
		Admin:  wc.CosmwasmClient.MustGetAddr(),
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
	resp, err := wc.CosmwasmClient.ListCodes(pagination)
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
	return wc.CosmwasmClient.ListContractsByCode(codeID, pagination)
}
