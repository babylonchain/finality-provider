package clientcontroller

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/config"
	"github.com/babylonchain/btc-validator/types"
)

var _ ClientController = &BabylonController{}

type BabylonController struct {
	bbnClient *bbnclient.Client
	cfg       *config.BBNConfig
	btcParams *chaincfg.Params
	logger    *logrus.Logger
}

func NewBabylonController(
	cfg *config.BBNConfig,
	btcParams *chaincfg.Params,
	logger *logrus.Logger,
) (*BabylonController, error) {

	bbnConfig := config.BBNConfigToBabylonConfig(cfg)

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
			return nil, fmt.Errorf("invalida covenant public key")
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
	}, nil
}

func (bc *BabylonController) reliablySendMsg(msg sdk.Msg) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg})
}

func (bc *BabylonController) reliablySendMsgs(msgs []sdk.Msg) (*provider.RelayerTxResponse, error) {
	return bc.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrors,
		unrecoverableErrors,
	)
}

// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterValidator(
	chainPk []byte,
	valPk *btcec.PublicKey,
	pop []byte,
	commission *big.Int,
	description []byte,
) (*types.TxResponse, error) {
	var bbnPop btcstakingtypes.ProofOfPossession
	if err := bbnPop.Unmarshal(pop); err != nil {
		return nil, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	sdkCommission := math.LegacyNewDecFromBigInt(commission)

	var sdkDescription sttypes.Description
	if err := sdkDescription.Unmarshal(description); err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	msg := &btcstakingtypes.MsgCreateBTCValidator{
		Signer:      bc.mustGetTxSigner(),
		BabylonPk:   &secp256k1.PubKey{Key: chainPk},
		BtcPk:       bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		Pop:         &bbnPop,
		Commission:  &sdkCommission,
		Description: &sdkDescription,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(
	valPk *btcec.PublicKey,
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
		ValBtcPk:    bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		StartHeight: startHeight,
		PubRandList: schnorrPubRandList,
		Sig:         &bip340Sig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitCovenantSigs submits the Covenant signature via a MsgAddCovenantSig to Babylon if the daemon runs in Covenant mode
// it returns tx hash and error
func (bc *BabylonController) SubmitCovenantSigs(
	covPk *btcec.PublicKey,
	stakingTxHash string,
	sigs [][]byte,
) (*types.TxResponse, error) {

	msg := &btcstakingtypes.MsgAddCovenantSig{
		Signer:        bc.mustGetTxSigner(),
		Pk:            bbntypes.NewBIP340PubKeyFromBTCPK(covPk),
		StakingTxHash: stakingTxHash,
		Sigs:          sigs,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitCovenantUnbondingSigs submits the Covenant signatures via a MsgAddCovenantUnbondingSigs to Babylon if the daemon runs in Covenant mode
// it returns tx hash and error
func (bc *BabylonController) SubmitCovenantUnbondingSigs(
	covPk *btcec.PublicKey,
	stakingTxHash string,
	unbondingSig *schnorr.Signature,
	slashUnbondingSigs [][]byte,
) (*types.TxResponse, error) {
	bip340UnbondingSig := bbntypes.NewBIP340SignatureFromBTCSig(unbondingSig)

	msg := &btcstakingtypes.MsgAddCovenantUnbondingSigs{
		Signer:                  bc.mustGetTxSigner(),
		Pk:                      bbntypes.NewBIP340PubKeyFromBTCPK(covPk),
		StakingTxHash:           stakingTxHash,
		UnbondingTxSig:          &bip340UnbondingSig,
		SlashingUnbondingTxSigs: slashUnbondingSigs,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(valPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {
	msg := &finalitytypes.MsgAddFinalitySig{
		Signer:       bc.mustGetTxSigner(),
		ValBtcPk:     bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		BlockHeight:  blockHeight,
		BlockAppHash: blockHash,
		FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sig),
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (bc *BabylonController) SubmitBatchFinalitySigs(valPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, b := range blocks {
		msg := &finalitytypes.MsgAddFinalitySig{
			Signer:       bc.mustGetTxSigner(),
			ValBtcPk:     bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
			BlockHeight:  b.Height,
			BlockAppHash: b.Hash,
			FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]),
		}
		msgs = append(msgs, msg)
	}

	res, err := bc.reliablySendMsgs(msgs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

func (bc *BabylonController) QueryPendingDelegations(limit uint64) ([]*types.Delegation, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_PENDING, limit)
}

func (bc *BabylonController) QueryUnbondingDelegations(limit uint64) ([]*types.Delegation, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_UNBONDING, limit)
}

// queryDelegationsWithStatus queries BTC delegations that need a Covenant signature
// with the given status (either pending or unbonding)
// it is only used when the program is running in Covenant mode
func (bc *BabylonController) queryDelegationsWithStatus(status btcstakingtypes.BTCDelegationStatus, limit uint64) ([]*types.Delegation, error) {
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	res, err := bc.bbnClient.QueryClient.BTCDelegations(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	dels := make([]*types.Delegation, 0, len(res.BtcDelegations))
	for _, d := range res.BtcDelegations {
		dels = append(dels, ConvertDelegationType(d))
	}

	return dels, nil
}

func (bc *BabylonController) QueryValidatorSlashed(valPk *btcec.PublicKey) (bool, error) {
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}
	valPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(valPk)

	queryRequest := &btcstakingtypes.QueryBTCValidatorRequest{ValBtcPkHex: valPubKey.MarshalHex()}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)
	res, err := queryClient.BTCValidator(ctx, queryRequest)
	if err != nil {
		return false, fmt.Errorf("failed to query the validator %s: %v", valPubKey.MarshalHex(), err)
	}

	slashed := res.BtcValidator.SlashedBtcHeight > 0

	return slashed, nil
}

func (bc *BabylonController) getNDelegations(
	valBtcPk *bbntypes.BIP340PubKey,
	startKey []byte,
	n uint64,
) ([]*types.Delegation, []byte, error) {
	pagination := &sdkquery.PageRequest{
		Key:   startKey,
		Limit: n,
	}

	res, err := bc.bbnClient.QueryClient.BTCValidatorDelegations(valBtcPk.MarshalHex(), pagination)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	var delegations []*types.Delegation

	for _, dels := range res.BtcDelegatorDelegations {
		for _, d := range dels.Dels {
			delegations = append(delegations, ConvertDelegationType(d))
		}
	}

	var nextKey []byte

	if res.Pagination != nil && res.Pagination.NextKey != nil {
		nextKey = res.Pagination.NextKey
	}

	return delegations, nextKey, nil
}

func (bc *BabylonController) getNValidatorDelegationsMatchingCriteria(
	valBtcPk *bbntypes.BIP340PubKey,
	n uint64,
	match func(*types.Delegation) bool,
) ([]*types.Delegation, error) {
	batchSize := 100
	var delegations []*types.Delegation
	var startKey []byte

	for {
		dels, nextKey, err := bc.getNDelegations(valBtcPk, startKey, uint64(batchSize))
		if err != nil {
			return nil, err
		}

		for _, del := range dels {
			if match(del) {
				delegations = append(delegations, del)
			}
		}

		if len(delegations) >= int(n) || len(nextKey) == 0 {
			break
		}

		startKey = nextKey
	}

	if len(delegations) > int(n) {
		// only return requested number of delegations
		return delegations[:n], nil
	} else {
		return delegations, nil
	}
}

// QueryValidatorVotingPower queries the voting power of the validator at a given height
func (bc *BabylonController) QueryValidatorVotingPower(valPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	res, err := bc.bbnClient.QueryClient.BTCValidatorPowerAtHeight(
		bbntypes.NewBIP340PubKeyFromBTCPK(valPk).MarshalHex(),
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
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	// query the indexed block at the given height
	queryRequest := &finalitytypes.QueryBlockRequest{
		Height: height,
	}
	res, err := queryClient.Block(ctx, queryRequest)
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
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query the indexed block at the given height
	queryRequest := &btcstakingtypes.QueryActivatedHeightRequest{}
	res, err := queryClient.ActivatedHeight(ctx, queryRequest)
	if err != nil {
		return 0, fmt.Errorf("failed to query activated height: %w", err)
	}

	return res.Height, nil
}

func (bc *BabylonController) QueryBestBlock() (*types.BlockInfo, error) {
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

func ConvertDelegationType(del *btcstakingtypes.BTCDelegation) *types.Delegation {
	var (
		stakingTxHex  string
		slashingTxHex string
		covenantSigs  []*types.CovenantAdaptorSigInfo
		undelegation  *types.Undelegation
	)

	if del.StakingTx == nil {
		panic(fmt.Errorf("staking tx should not be empty in delegation"))
	}

	if del.SlashingTx == nil {
		panic(fmt.Errorf("slashing tx should not be empty in delegation"))
	}

	stakingTxHex = hex.EncodeToString(del.StakingTx)

	slashingTxHex = del.SlashingTx.ToHexStr()

	for _, s := range del.CovenantSigs {
		covSigInfo := &types.CovenantAdaptorSigInfo{
			Pk:   s.CovPk.MustToBTCPK(),
			Sigs: s.AdaptorSigs,
		}
		covenantSigs = append(covenantSigs, covSigInfo)
	}

	if del.BtcUndelegation != nil {
		undelegation = ConvertUndelegationType(del.BtcUndelegation)
	}

	valBtcPks := make([]*btcec.PublicKey, 0, len(del.ValBtcPkList))
	for _, val := range del.ValBtcPkList {
		valBtcPks = append(valBtcPks, val.MustToBTCPK())
	}

	return &types.Delegation{
		BtcPk:           del.BtcPk.MustToBTCPK(),
		ValBtcPks:       valBtcPks,
		TotalSat:        del.TotalSat,
		StartHeight:     del.StartHeight,
		EndHeight:       del.EndHeight,
		StakingTxHex:    stakingTxHex,
		SlashingTxHex:   slashingTxHex,
		CovenantSigs:    covenantSigs,
		BtcUndelegation: undelegation,
	}
}

func ConvertUndelegationType(undel *btcstakingtypes.BTCUndelegation) *types.Undelegation {
	var (
		unbondingTxHex        string
		slashingTxHex         string
		covenantSlashingSigs  []*types.CovenantAdaptorSigInfo
		covenantUnbondingSigs []*types.CovenantSchnorrSigInfo
	)

	if undel.UnbondingTx == nil {
		panic(fmt.Errorf("staking tx should not be empty in undelegation"))
	}

	if undel.SlashingTx == nil {
		panic(fmt.Errorf("slashing tx should not be empty in undelegation"))
	}

	unbondingTxHex = hex.EncodeToString(undel.UnbondingTx)

	slashingTxHex = undel.SlashingTx.ToHexStr()

	for _, unbondingSig := range undel.CovenantUnbondingSigList {
		sig, err := unbondingSig.Sig.ToBTCSig()
		if err != nil {
			panic(err)
		}
		sigInfo := &types.CovenantSchnorrSigInfo{
			Pk:  unbondingSig.Pk.MustToBTCPK(),
			Sig: sig,
		}
		covenantUnbondingSigs = append(covenantUnbondingSigs, sigInfo)
	}

	for _, s := range undel.CovenantSlashingSigs {
		covSigInfo := &types.CovenantAdaptorSigInfo{
			Pk:   s.CovPk.MustToBTCPK(),
			Sigs: s.AdaptorSigs,
		}
		covenantSlashingSigs = append(covenantSlashingSigs, covSigInfo)
	}

	return &types.Undelegation{
		UnbondingTxHex:        unbondingTxHex,
		SlashingTxHex:         slashingTxHex,
		CovenantSlashingSigs:  covenantSlashingSigs,
		CovenantUnbondingSigs: covenantUnbondingSigs,
		UnbondingTime:         undel.UnbondingTime,
	}
}

// Currently this is only used for e2e tests, probably does not need to add it into the interface
func (bc *BabylonController) CreateBTCDelegation(
	delBabylonPk *secp256k1.PubKey,
	delBtcPk *bbntypes.BIP340PubKey,
	valPks []*btcec.PublicKey,
	pop *btcstakingtypes.ProofOfPossession,
	stakingTime uint32,
	stakingValue int64,
	stakingTxInfo *btcctypes.TransactionInfo,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSig *bbntypes.BIP340Signature,
) (*types.TxResponse, error) {
	valBtcPks := make([]bbntypes.BIP340PubKey, 0, len(valPks))
	for _, v := range valPks {
		valBtcPks = append(valBtcPks, *bbntypes.NewBIP340PubKeyFromBTCPK(v))
	}
	msg := &btcstakingtypes.MsgCreateBTCDelegation{
		Signer:       bc.mustGetTxSigner(),
		BabylonPk:    delBabylonPk,
		BtcPk:        delBtcPk,
		ValBtcPkList: valBtcPks,
		Pop:          pop,
		StakingTime:  stakingTime,
		StakingValue: stakingValue,
		StakingTx:    stakingTxInfo,
		SlashingTx:   slashingTx,
		DelegatorSig: delSig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	bc.logger.Infof("successfully submitted a BTC delegation, code: %v, height: %v, tx hash: %s", res.Code, res.Height, res.TxHash)
	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) CreateBTCUndelegation(
	unbondingTx []byte,
	unbondingTime uint32,
	unbondingValue int64,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSig *bbntypes.BIP340Signature,
) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgBTCUndelegate{
		Signer:               bc.mustGetTxSigner(),
		UnbondingTx:          unbondingTx,
		UnbondingTime:        unbondingTime,
		UnbondingValue:       unbondingValue,
		SlashingTx:           slashingTx,
		DelegatorSlashingSig: delSig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	bc.logger.Infof("successfully submitted a BTC undelegation, code: %v, height: %v, tx hash: %s", res.Code, res.Height, res.TxHash)
	return res, nil
}

// Insert BTC block header using rpc client
// Currently this is only used for e2e tests, probably does not need to add it into the interface
func (bc *BabylonController) InsertBtcBlockHeaders(headers []bbntypes.BTCHeaderBytes) (*provider.RelayerTxResponse, error) {
	msg := &btclctypes.MsgInsertHeaders{
		Signer:  bc.mustGetTxSigner(),
		Headers: headers,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryValidators queries BTC validators
// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryValidators() ([]*btcstakingtypes.BTCValidator, error) {
	var validators []*btcstakingtypes.BTCValidator
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	for {
		queryRequest := &btcstakingtypes.QueryBTCValidatorsRequest{
			Pagination: pagination,
		}
		res, err := queryClient.BTCValidators(ctx, queryRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to query BTC validators: %v", err)
		}
		validators = append(validators, res.BtcValidators...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return validators, nil
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryBtcLightClientTip() (*btclctypes.BTCHeaderInfo, error) {
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}

	queryClient := btclctypes.NewQueryClient(clientCtx)

	queryRequest := &btclctypes.QueryTipRequest{}
	res, err := queryClient.Tip(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC tip: %v", err)
	}

	return res.Header, nil
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryBTCValidatorDelegations(valBtcPk *bbntypes.BIP340PubKey, max uint64) ([]*types.Delegation, error) {
	return bc.getNValidatorDelegationsMatchingCriteria(
		valBtcPk,
		max,
		// fitlering function which always returns true as we want all delegations
		func(*types.Delegation) bool { return true },
	)
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryVotesAtHeight(height uint64) ([]bbntypes.BIP340PubKey, error) {
	ctx, cancel := getContextWithCancel(bc.cfg.Timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.bbnClient.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &finalitytypes.QueryVotesAtHeightRequest{
		Height: height,
	}
	res, err := queryClient.VotesAtHeight(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.BtcPks, nil
}
