package clientcontroller

import (
	"context"
	"fmt"
	"time"

	sdkErr "cosmossdk.io/errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"

	"cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
)

var _ ClientController = &BabylonController{}

var emptyErrs = []*sdkErr.Error{}

type BabylonController struct {
	bbnClient *bbnclient.Client
	cfg       *fpcfg.BBNConfig
	btcParams *chaincfg.Params
	logger    *zap.Logger
}

func NewBabylonController(
	cfg *fpcfg.BBNConfig,
	btcParams *chaincfg.Params,
	logger *zap.Logger,
) (*BabylonController, error) {

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

	return &BabylonController{
		bc,
		cfg,
		btcParams,
		logger,
	}, nil
}

func (bc *BabylonController) mustGetTxSigner() string {
	signer := bc.GetKeyAddress()
	prefix := bc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (bc *BabylonController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonController, so if it fails something is really wrong

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

func (bc *BabylonController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (bc *BabylonController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// RegisterFinalityProvider registers a finality provider via a MsgCreateFinalityProvider to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterFinalityProvider(
	chainID string,
	chainPk []byte,
	fpPk *btcec.PublicKey,
	pop []byte,
	commission *math.LegacyDec,
	description []byte,
) (*types.TxResponse, error) {
	var bbnPop btcstakingtypes.ProofOfPossession
	if err := bbnPop.Unmarshal(pop); err != nil {
		return nil, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	var sdkDescription sttypes.Description
	if err := sdkDescription.Unmarshal(description); err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	msg := &btcstakingtypes.MsgCreateFinalityProvider{
		Signer:      bc.mustGetTxSigner(),
		BabylonPk:   &secp256k1.PubKey{Key: chainPk},
		BtcPk:       bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		Pop:         &bbnPop,
		Commission:  commission,
		Description: &sdkDescription,
		ChainId:     chainID,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	pubRandList []*btcec.FieldVal,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	schnorrPubRandList := make([]bbntypes.SchnorrPubRand, 0, len(pubRandList))
	for _, r := range pubRandList {
		schnorrPubRand := bbntypes.NewSchnorrPubRandFromFieldVal(r)
		schnorrPubRandList = append(schnorrPubRandList, *schnorrPubRand)
	}

	bip340Sig := bbntypes.NewBIP340SignatureFromBTCSig(sig)

	msg := &finalitytypes.MsgCommitPubRandList{
		Signer:      bc.mustGetTxSigner(),
		FpBtcPk:     bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		StartHeight: startHeight,
		PubRandList: schnorrPubRandList,
		Sig:         bip340Sig,
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidPubRand,
		finalitytypes.ErrTooFewPubRand,
		finalitytypes.ErrNoPubRandYet,
		btcstakingtypes.ErrFpNotFound,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(fpPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {
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
func (bc *BabylonController) SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
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

func (bc *BabylonController) QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error) {
	fpPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)
	res, err := bc.bbnClient.QueryClient.FinalityProvider(fpPubKey.MarshalHex())
	if err != nil {
		return false, fmt.Errorf("failed to query the finality provider %s: %v", fpPubKey.MarshalHex(), err)
	}

	slashed := res.FinalityProvider.SlashedBtcHeight > 0

	return slashed, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (bc *BabylonController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProviderPowerAtHeight(
		bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
		blockHeight,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.VotingPower, nil
}

func (bc *BabylonController) QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error) {
	return bc.queryLatestBlocks(nil, count, finalitytypes.QueriedBlockStatus_FINALIZED, true)
}

// QueryLastCommittedPublicRand returns the last committed public randomness
// TODO update the implementation when rpc-client supports ListPublicRandomness
func (bc *BabylonController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*btcec.FieldVal, error) {
	fpBtcPk := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)

	pagination := &sdkquery.PageRequest{
		// NOTE: the count is limited by pagination queries
		Limit:   count,
		Reverse: true,
	}

	res, err := bc.bbnClient.QueryClient.ListPublicRandomness(fpBtcPk.MarshalHex(), pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query committed public randomness: %w", err)
	}

	committedPubRand := make(map[uint64]*btcec.FieldVal, len(res.PubRandMap))
	for k, v := range res.PubRandMap {
		if v == nil {
			return nil, fmt.Errorf("invalid committed public randomness")
		}
		committedPubRand[k] = v.ToFieldVal()
	}

	return committedPubRand, nil
}

func (bc *BabylonController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	if endHeight < startHeight {
		return nil, fmt.Errorf("the startHeight %v should not be higher than the endHeight %v", startHeight, endHeight)
	}
	count := endHeight - startHeight + 1
	if count > limit {
		count = limit
	}
	return bc.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), count, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (bc *BabylonController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
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

func (bc *BabylonController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	res, err := bc.bbnClient.QueryClient.Block(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed block at height %v: %w", height, err)
	}

	return &types.BlockInfo{
		Height:    height,
		Hash:      res.Block.AppHash,
		Finalized: res.Block.Finalized,
	}, nil
}

func (bc *BabylonController) QueryActivatedHeight() (uint64, error) {
	res, err := bc.bbnClient.QueryClient.ActivatedHeight()
	if err != nil {
		return 0, fmt.Errorf("failed to query activated height: %w", err)
	}

	return res.Height, nil
}

func (bc *BabylonController) QueryBestBlock() (*types.BlockInfo, error) {
	blocks, err := bc.queryLatestBlocks(nil, 1, finalitytypes.QueriedBlockStatus_ANY, true)
	if err != nil || len(blocks) != 1 {
		// try query comet block if the index block query is not available
		return bc.queryCometBestBlock()
	}

	return blocks[0], nil
}

func (bc *BabylonController) queryCometBestBlock() (*types.BlockInfo, error) {
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

func (bc *BabylonController) Close() error {
	if !bc.bbnClient.IsRunning() {
		return nil
	}

	return bc.bbnClient.Stop()
}

/*
	Implementations for e2e tests only
*/

func (bc *BabylonController) CreateBTCDelegation(
	delBabylonPk *secp256k1.PubKey,
	delBtcPk *bbntypes.BIP340PubKey,
	fpPks []*btcec.PublicKey,
	pop *btcstakingtypes.ProofOfPossession,
	stakingTime uint32,
	stakingValue int64,
	stakingTxInfo *btcctypes.TransactionInfo,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSlashingSig *bbntypes.BIP340Signature,
	unbondingTx []byte,
	unbondingTime uint32,
	unbondingValue int64,
	unbondingSlashingTx *btcstakingtypes.BTCSlashingTx,
	delUnbondingSlashingSig *bbntypes.BIP340Signature,
) (*types.TxResponse, error) {
	fpBtcPks := make([]bbntypes.BIP340PubKey, 0, len(fpPks))
	for _, v := range fpPks {
		fpBtcPks = append(fpBtcPks, *bbntypes.NewBIP340PubKeyFromBTCPK(v))
	}
	msg := &btcstakingtypes.MsgCreateBTCDelegation{
		Signer:                        bc.mustGetTxSigner(),
		BabylonPk:                     delBabylonPk,
		Pop:                           pop,
		BtcPk:                         delBtcPk,
		FpBtcPkList:                   fpBtcPks,
		StakingTime:                   stakingTime,
		StakingValue:                  stakingValue,
		StakingTx:                     stakingTxInfo,
		SlashingTx:                    slashingTx,
		DelegatorSlashingSig:          delSlashingSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingSlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) InsertBtcBlockHeaders(headers []bbntypes.BTCHeaderBytes) (*provider.RelayerTxResponse, error) {
	msg := &btclctypes.MsgInsertHeaders{
		Signer:  bc.mustGetTxSigner(),
		Headers: headers,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (bc *BabylonController) QueryFinalityProviders() ([]*btcstakingtypes.FinalityProviderResponse, error) {
	var fps []*btcstakingtypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	for {
		res, err := bc.bbnClient.QueryClient.FinalityProviders(pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %v", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func (bc *BabylonController) QueryBtcLightClientTip() (*btclctypes.BTCHeaderInfoResponse, error) {
	res, err := bc.bbnClient.QueryClient.BTCHeaderChainTip()
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC tip: %v", err)
	}

	return res.Header, nil
}

func (bc *BabylonController) QueryVotesAtHeight(height uint64) ([]bbntypes.BIP340PubKey, error) {
	res, err := bc.bbnClient.QueryClient.VotesAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.BtcPks, nil
}

func (bc *BabylonController) QueryPendingDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_PENDING, limit)
}

func (bc *BabylonController) QueryActiveDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_ACTIVE, limit)
}

// queryDelegationsWithStatus queries BTC delegations
// with the given status (either pending or unbonding)
// it is only used when the program is running in Covenant mode
func (bc *BabylonController) queryDelegationsWithStatus(status btcstakingtypes.BTCDelegationStatus, limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	res, err := bc.bbnClient.QueryClient.BTCDelegations(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	return res.BtcDelegations, nil
}

func (bc *BabylonController) QueryStakingParams() (*types.StakingParams, error) {
	// query btc checkpoint params
	ckptParamRes, err := bc.bbnClient.QueryClient.BTCCheckpointParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query params of the btccheckpoint module: %v", err)
	}

	// query btc staking params
	stakingParamRes, err := bc.bbnClient.QueryClient.BTCStakingParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query staking params: %v", err)
	}

	covenantPks := make([]*btcec.PublicKey, 0, len(stakingParamRes.Params.CovenantPks))
	for _, pk := range stakingParamRes.Params.CovenantPks {
		covPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, fmt.Errorf("invalid covenant public key")
		}
		covenantPks = append(covenantPks, covPk)
	}
	slashingAddress, err := btcutil.DecodeAddress(stakingParamRes.Params.SlashingAddress, bc.btcParams)
	if err != nil {
		return nil, err
	}

	return &types.StakingParams{
		ComfirmationTimeBlocks:    ckptParamRes.Params.BtcConfirmationDepth,
		FinalizationTimeoutBlocks: ckptParamRes.Params.CheckpointFinalizationTimeout,
		MinSlashingTxFeeSat:       btcutil.Amount(stakingParamRes.Params.MinSlashingTxFeeSat),
		CovenantPks:               covenantPks,
		SlashingAddress:           slashingAddress,
		CovenantQuorum:            stakingParamRes.Params.CovenantQuorum,
		SlashingRate:              stakingParamRes.Params.SlashingRate,
		MinUnbondingTime:          stakingParamRes.Params.MinUnbondingTime,
	}, nil
}

func (bc *BabylonController) SubmitCovenantSigs(
	covPk *btcec.PublicKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *schnorr.Signature,
	unbondingSlashingSigs [][]byte,
) (*types.TxResponse, error) {
	bip340UnbondingSig := bbntypes.NewBIP340SignatureFromBTCSig(unbondingSig)

	msg := &btcstakingtypes.MsgAddCovenantSigs{
		Signer:                  bc.mustGetTxSigner(),
		Pk:                      bbntypes.NewBIP340PubKeyFromBTCPK(covPk),
		StakingTxHash:           stakingTxHash,
		SlashingTxSigs:          slashingSigs,
		UnbondingTxSig:          bip340UnbondingSig,
		SlashingUnbondingTxSigs: unbondingSlashingSigs,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// RegisterConsumerChain registers a consumer chain via a MsgRegisterChain to Babylon
func (bc *BabylonController) RegisterConsumerChain(id, name, description string) (*types.TxResponse, error) {
	msg := &bsctypes.MsgRegisterChain{
		Signer:           bc.mustGetTxSigner(),
		ChainId:          id,
		ChainName:        name,
		ChainDescription: description,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}
