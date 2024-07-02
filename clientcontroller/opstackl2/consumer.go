package opstackl2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbnapp "github.com/babylonchain/babylon/app"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/clientcontroller/api"
	cwclient "github.com/babylonchain/finality-provider/cosmwasmclient/client"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
)

const (
	BabylonChainName = "Babylon"
)

var _ api.ConsumerController = &OPStackL2ConsumerController{}

type OPStackL2ConsumerController struct {
	Cfg        *fpcfg.OPStackL2Config
	CwClient   *cwclient.Client
	opl2Client *ethclient.Client
	logger     *zap.Logger
}

func NewOPStackL2ConsumerController(
	opl2Cfg *fpcfg.OPStackL2Config,
	logger *zap.Logger,
) (*OPStackL2ConsumerController, error) {
	if opl2Cfg == nil {
		return nil, fmt.Errorf("nil config for OP consumer controller")
	}
	cwConfig := opl2Cfg.ToCosmwasmConfig()
	if err := cwConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for OP consumer controller: %w", err)
	}

	bbnEncodingCfg := bbnapp.GetEncodingConfig()
	cwEncodingCfg := wasmdparams.EncodingConfig{
		InterfaceRegistry: bbnEncodingCfg.InterfaceRegistry,
		Codec:             bbnEncodingCfg.Codec,
		TxConfig:          bbnEncodingCfg.TxConfig,
		Amino:             bbnEncodingCfg.Amino,
	}

	cwClient, err := cwclient.New(
		&cwConfig,
		BabylonChainName,
		cwEncodingCfg,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opl2Cfg.Timeout)
	defer cancel()

	opl2Client, err := ethclient.DialContext(ctx, opl2Cfg.OPStackL2RPCAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create OPStack L2 client: %w", err)
	}

	return &OPStackL2ConsumerController{
		opl2Cfg,
		cwClient,
		opl2Client,
		logger,
	}, nil
}

func (cc *OPStackL2ConsumerController) ReliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return cc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (cc *OPStackL2ConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return cc.CwClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// CommitPubRandList commits a list of Schnorr public randomness to Babylon CosmWasm contract
// it returns tx hash and error
func (cc *OPStackL2ConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	msg := CommitPublicRandomnessMsg{
		CommitPublicRandomness: CommitPublicRandomnessMsgParams{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig.Serialize(),
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	execMsg := &wasmtypes.MsgExecuteContract{
		Sender:   cc.CwClient.MustGetAddr(),
		Contract: cc.Cfg.OPFinalityGadgetAddress,
		Msg:      payload,
	}

	res, err := cc.ReliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return nil, err
	}
	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitFinalitySig submits the finality signature to Babylon CosmWasm contract
// it returns tx hash and error
func (cc *OPStackL2ConsumerController) SubmitFinalitySig(
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

	msg := SubmitFinalitySignatureMsg{
		SubmitFinalitySignature: SubmitFinalitySignatureMsgParams{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			Height:      block.Height,
			PubRand:     bbntypes.NewSchnorrPubRandFromFieldVal(pubRand).MustMarshal(),
			Proof:       ConvertProof(cmtProof),
			BlockHash:   block.Hash,
			Signature:   bbntypes.NewSchnorrEOTSSigFromModNScalar(sig).MustMarshal(),
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	execMsg := &wasmtypes.MsgExecuteContract{
		Sender:   cc.CwClient.MustGetAddr(),
		Contract: cc.Cfg.OPFinalityGadgetAddress,
		Msg:      payload,
	}

	res, err := cc.ReliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return nil, err
	}
	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures
func (cc *OPStackL2ConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*types.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}
	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, block := range blocks {
		cmtProof := cmtcrypto.Proof{}
		if err := cmtProof.Unmarshal(proofList[i]); err != nil {
			return nil, err
		}

		msg := SubmitFinalitySignatureMsg{
			SubmitFinalitySignature: SubmitFinalitySignatureMsgParams{
				FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
				Height:      block.Height,
				PubRand:     bbntypes.NewSchnorrPubRandFromFieldVal(pubRandList[i]).MustMarshal(),
				Proof:       ConvertProof(cmtProof),
				BlockHash:   block.Hash,
				Signature:   bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]).MustMarshal(),
			},
		}
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		execMsg := &wasmtypes.MsgExecuteContract{
			Sender:   cc.CwClient.MustGetAddr(),
			Contract: cc.Cfg.OPFinalityGadgetAddress,
			Msg:      payload,
		}
		msgs = append(msgs, execMsg)
	}

	res, err := cc.reliablySendMsgs(msgs, nil, nil)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height.
func (cc *OPStackL2ConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	res, err := cc.QueryPublicRandCommit(fpPk, blockHeight)
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, nil
	}
	// This interface function only used for checking if the FP is eligible for submitting sigs.
	// Now we can simply hardcode the voting power to a positive value.
	// TODO: see this issue https://github.com/babylonchain/finality-provider/issues/390 for more details
	return 1, nil
}

// QueryLatestFinalizedBlock returns the finalized L2 block from a RPC call
// TODO: return the BTC finalized L2 block, it is tricky b/c it's not recorded anywhere so we can
// use some exponential strategy to search
func (cc *OPStackL2ConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cc.Cfg.Timeout)
	defer cancel()

	l2Block, err := cc.opl2Client.HeaderByNumber(ctx, big.NewInt(ethrpc.FinalizedBlockNumber.Int64()))
	if err != nil {
		return nil, err
	}
	return &types.BlockInfo{
		Height: l2Block.Number.Uint64(),
		Hash:   l2Block.Hash().Bytes(),
	}, nil
}

func (cc *OPStackL2ConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo
	var count uint64 = 0

	// TODO(lester): add test
	for height := startHeight; height <= endHeight && count < limit; height++ {
		block, err := cc.QueryBlock(height)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
		count++
	}
	return blocks, nil
}

// QueryBlock returns the L2 block number and block hash with the given block number from a RPC call
func (cc *OPStackL2ConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cc.Cfg.Timeout)
	defer cancel()

	l2Block, err := cc.opl2Client.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil {
		return nil, err
	}
	return &types.BlockInfo{
		Height: height,
		Hash:   l2Block.Hash().Bytes(),
	}, nil
}

// QueryIsBlockFinalized returns whether the given the L2 block number has been finalized
func (cc *OPStackL2ConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	l2Block, err := cc.QueryLatestFinalizedBlock()
	if err != nil {
		return false, err
	}
	if height > l2Block.Height {
		return false, nil
	}
	return true, nil
}

// QueryActivatedHeight returns the L2 block number at which the finality gadget is activated.
// It is fetched from the configuration of a CosmWasm contract OP finality gadget.
func (cc *OPStackL2ConsumerController) QueryActivatedHeight() (uint64, error) {
	queryMsg := &QueryMsg{Config: &Config{}}
	jsonData, err := json.Marshal(queryMsg) // `{"config":{}}`
	if err != nil {
		return 0, fmt.Errorf("failed marshaling to JSON: %w", err)
	}
	stateResp, err := cc.CwClient.QuerySmartContractState(cc.Cfg.OPFinalityGadgetAddress, string(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	var resp ConfigResponse
	err = json.Unmarshal(stateResp.Data, &resp)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return resp.ActivatedHeight, nil
}

// QueryLatestBlockHeight gets the latest L2 block number from a RPC call
func (cc *OPStackL2ConsumerController) QueryLatestBlockHeight() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cc.Cfg.Timeout)
	defer cancel()

	l2LatestBlock, err := cc.opl2Client.HeaderByNumber(ctx, big.NewInt(ethrpc.LatestBlockNumber.Int64()))
	if err != nil {
		return 0, err
	}

	return l2LatestBlock.Number.Uint64(), nil
}

// QueryLastPublicRandCommit returns the last public randomness commitments
// It is fetched from the state of a CosmWasm contract OP finality gadget.
func (cc *OPStackL2ConsumerController) QueryLastPublicRandCommit(fpPk *btcec.PublicKey) (*types.PubRandCommit, error) {
	fpPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)
	queryMsg := &QueryMsg{
		LastPubRandCommit: &LastPubRandCommit{
			BtcPkHex: fpPubKey.MarshalHex(),
		},
	}

	jsonData, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling to JSON: %w", err)
	}

	stateResp, err := cc.CwClient.QuerySmartContractState(cc.Cfg.OPFinalityGadgetAddress, string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// If CosmWasm contract's return data is None, the corresponding JSON representation is a four-character string "null"
	// and the json.Unmarshal() does NOT raise an error, we should explicitly check for this condition
	if stateResp.Data == nil || string(stateResp.Data) == "null" {
		return nil, nil
	}

	var resp *types.PubRandCommit
	err = json.Unmarshal(stateResp.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if err := resp.Validate(); err != nil {
		return nil, err
	}

	return resp, nil
}

func (cc *OPStackL2ConsumerController) QueryPublicRandCommit(fpPk *btcec.PublicKey, height uint64) (*types.PubRandCommit, error) {
	queryMsg := &QueryMsg{PubRandCommit: &PubRandCommit{
		BtcPkHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
		Height:   height,
	}}
	jsonData, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling to JSON: %w", err)
	}
	stateResp, err := cc.CwClient.QuerySmartContractState(cc.Cfg.OPFinalityGadgetAddress, string(jsonData))
	if err != nil {
		return nil, nil
	}
	if stateResp.Data == nil || string(stateResp.Data) == "null" {
		return nil, nil
	}

	var resp *types.PubRandCommit
	err = json.Unmarshal(stateResp.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if err := resp.Validate(); err != nil {
		return nil, err
	}

	return resp, nil
}

func (cc *OPStackL2ConsumerController) QueryFirstPublicRandCommit(fpPk *btcec.PublicKey) (*types.PubRandCommit, error) {
	fpPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)
	queryMsg := &QueryMsg{
		FirstPubRandCommit: &FirstPubRandCommit{
			BtcPkHex: fpPubKey.MarshalHex(),
		},
	}

	jsonData, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling to JSON: %w", err)
	}

	stateResp, err := cc.CwClient.QuerySmartContractState(cc.Cfg.OPFinalityGadgetAddress, string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}

	// If CosmWasm contract's return data is None, the corresponding JSON representation is a four-character string "null"
	// and the json.Unmarshal() does NOT raise an error, we should explicitly check for this condition
	if stateResp.Data == nil || string(stateResp.Data) == "null" {
		return nil, nil
	}

	var resp *types.PubRandCommit
	err = json.Unmarshal(stateResp.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if err := resp.Validate(); err != nil {
		return nil, err
	}

	return resp, nil
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

func (cc *OPStackL2ConsumerController) Close() error {
	cc.opl2Client.Close()
	return cc.CwClient.Stop()
}
