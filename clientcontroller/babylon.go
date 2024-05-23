package clientcontroller

import (
	"context"
	"fmt"

	sdkErr "cosmossdk.io/errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"

	"cosmossdk.io/math"
	bbnclient "github.com/babylonchain/babylon/client/client"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
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
// it returns tx hash, registered epoch, and error
func (bc *BabylonController) RegisterFinalityProvider(
	chainID string,
	chainPk []byte,
	fpPk *btcec.PublicKey,
	pop []byte,
	commission *math.LegacyDec,
	description []byte,
	masterPubRand string,
) (*types.TxResponse, uint64, error) {
	var bbnPop btcstakingtypes.ProofOfPossession
	if err := bbnPop.Unmarshal(pop); err != nil {
		return nil, 0, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	var sdkDescription sttypes.Description
	if err := sdkDescription.Unmarshal(description); err != nil {
		return nil, 0, fmt.Errorf("invalid description: %w", err)
	}

	msg := &btcstakingtypes.MsgCreateFinalityProvider{
		Signer:        bc.mustGetTxSigner(),
		BabylonPk:     &secp256k1.PubKey{Key: chainPk},
		BtcPk:         bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		Pop:           &bbnPop,
		Commission:    commission,
		Description:   &sdkDescription,
		MasterPubRand: masterPubRand,
		ConsumerId:    chainID,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, 0, err
	}

	registeredEpoch, err := bc.QueryFinalityProviderRegisteredEpoch(fpPk)
	if err != nil {
		return nil, 0, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, registeredEpoch, nil
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

// QueryFinalityProviderRegisteredEpoch queries the registered epoch of the finality provider
func (bc *BabylonController) QueryFinalityProviderRegisteredEpoch(fpPk *btcec.PublicKey) (uint64, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProvider(
		bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query finality provider registered epoch: %w", err)
	}

	return res.FinalityProvider.RegisteredEpoch, nil
}

func (bc *BabylonController) QueryLastFinalizedEpoch() (uint64, error) {
	resp, err := bc.bbnClient.LatestEpochFromStatus(ckpttypes.Finalized)
	if err != nil {
		return 0, err
	}
	return resp.RawCheckpoint.EpochNum, nil
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

func (bc *BabylonController) GetBBNClient() *bbnclient.Client {
	return bc.bbnClient
}

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

func (bc *BabylonController) InsertSpvProofs(submitter string, proofs []*btcctypes.BTCSpvProof) (*provider.RelayerTxResponse, error) {
	msg := &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter,
		Proofs:    proofs,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// RegisterConsumerChain registers a consumer chain via a MsgRegisterChain to Babylon
func (bc *BabylonController) RegisterConsumerChain(id, name, description string) (*types.TxResponse, error) {
	msg := &bsctypes.MsgRegisterConsumer{
		Signer:              bc.mustGetTxSigner(),
		ConsumerId:          id,
		ConsumerName:        name,
		ConsumerDescription: description,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}
