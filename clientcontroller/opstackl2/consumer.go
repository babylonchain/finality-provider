package opstackl2

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	sdkErr "cosmossdk.io/errors"
	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbnapp "github.com/babylonchain/babylon/app"
	bbntypes "github.com/babylonchain/babylon/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
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
	"go.uber.org/zap"
)

const (
	BabylonChainName = "Babylon"
)

var _ api.ConsumerController = &OPStackL2ConsumerController{}

type OPStackL2ConsumerController struct {
	bbnClient  *cwclient.Client
	opl2Client *ethclient.Client
	cfg        *fpcfg.OPStackL2Config
	logger     *zap.Logger
}

func NewOPStackL2ConsumerController(
	opl2Cfg *fpcfg.OPStackL2Config,
	logger *zap.Logger,
) (*OPStackL2ConsumerController, error) {
	cwConfig := opl2Cfg.ToCosmwasmConfig()
	if err := cwConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon client: %w", err)
	}

	bbnEncodingCfg := bbnapp.GetEncodingConfig()
	wasmdEncodingCfg := wasmdparams.EncodingConfig{
		InterfaceRegistry: bbnEncodingCfg.InterfaceRegistry,
		Codec:             bbnEncodingCfg.Codec,
		TxConfig:          bbnEncodingCfg.TxConfig,
		Amino:             bbnEncodingCfg.Amino,
	}

	bbnClient, err := cwclient.New(
		&cwConfig,
		BabylonChainName,
		wasmdEncodingCfg,
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
		bbnClient,
		opl2Client,
		opl2Cfg,
		logger,
	}, nil
}

func (cc *OPStackL2ConsumerController) ExecuteContract(payload []byte) (*provider.RelayerTxResponse, error) {
	execMsg := &wasmtypes.MsgExecuteContract{
		Sender:   cc.bbnClient.MustGetAddr(),
		Contract: cc.cfg.OPFinalityGadgetAddress,
		Msg:      payload,
	}

	res, err := cc.reliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (cc *OPStackL2ConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return cc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (cc *OPStackL2ConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return cc.bbnClient.ReliablySendMsgs(
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
		Sender:   cc.bbnClient.MustGetAddr(),
		Contract: cc.cfg.OPFinalityGadgetAddress,
		Msg:      payload,
	}

	res, err := cc.reliablySendMsg(execMsg, nil, nil)
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
			Proof:       &cmtProof,
			BlockHash:   block.Hash,
			Signature:   bbntypes.NewSchnorrEOTSSigFromModNScalar(sig).MustMarshal(),
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	execMsg := &wasmtypes.MsgExecuteContract{
		Sender:   cc.bbnClient.MustGetAddr(),
		Contract: cc.cfg.OPFinalityGadgetAddress,
		Msg:      payload,
	}

	res, err := cc.reliablySendMsg(execMsg, nil, nil)
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
				Proof:       &cmtProof,
				BlockHash:   block.Hash,
				Signature:   bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]).MustMarshal(),
			},
		}
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		execMsg := &wasmtypes.MsgExecuteContract{
			Sender:   cc.bbnClient.MustGetAddr(),
			Contract: cc.cfg.OPFinalityGadgetAddress,
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

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (cc *OPStackL2ConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	return 0, nil
}

func (cc *OPStackL2ConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	return &types.BlockInfo{
		Height: 0,
		Hash:   nil,
	}, nil
}

func (cc *OPStackL2ConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	return cc.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), 0, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (cc *OPStackL2ConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo
	return blocks, nil
}

// QueryBlock gets the L2 block number and block hash with the given block number from a RPC call
func (cc *OPStackL2ConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cc.cfg.Timeout)
	defer cancel()

	l2Block, err := cc.opl2Client.BlockByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil {
		return nil, err
	}
	return &types.BlockInfo{
		Height: height,
		Hash:   l2Block.Hash().Bytes(),
	}, nil
}

func (cc *OPStackL2ConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	return false, nil
}

func (cc *OPStackL2ConsumerController) QueryActivatedHeight() (uint64, error) {
	return 0, nil
}

// QueryLatestBlockHeight gets the latest L2 block number from a RPC call
func (cc *OPStackL2ConsumerController) QueryLatestBlockHeight() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cc.cfg.Timeout)
	defer cancel()

	return cc.opl2Client.BlockNumber(ctx)
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (ec *OPStackL2ConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*types.PubRandCommit, error) {
	return nil, nil
}

func (cc *OPStackL2ConsumerController) Close() error {
	cc.opl2Client.Close()
	return cc.bbnClient.Stop()
}
