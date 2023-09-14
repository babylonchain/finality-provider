package clientcontroller

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	bbnapp "github.com/babylonchain/babylon/app"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcutil"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkTypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	pv "github.com/cosmos/relayer/v2/relayer/provider"
	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/juju/fslock"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"

	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

var _ ClientController = &BabylonController{}

type BabylonController struct {
	provider *cosmos.CosmosProvider
	logger   *logrus.Logger
	timeout  time.Duration
}

func newRootLogger(format string, debug bool) (*zap.Logger, error) {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	var enc zapcore.Encoder
	switch format {
	case "json":
		enc = zapcore.NewJSONEncoder(config)
	case "auto", "console":
		enc = zapcore.NewConsoleEncoder(config)
	case "logfmt":
		enc = zaplogfmt.NewEncoder(config)
	default:
		return nil, fmt.Errorf("unrecognized log format %q", format)
	}

	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}
	return zap.New(zapcore.NewCore(
		enc,
		os.Stderr,
		level,
	)), nil
}

func NewBabylonController(
	homedir string,
	cfg *valcfg.BBNConfig,
	logger *logrus.Logger,
) (*BabylonController, error) {

	zapLogger, err := newRootLogger("console", true)
	if err != nil {
		return nil, err
	}

	// HACK: replace the modules in public rpc-client to add BTC staking / finality modules
	// so that it recognises their message formats
	// TODO: fix this either by fixing rpc-client side
	var moduleBasics []module.AppModuleBasic
	for _, mbasic := range bbnapp.ModuleBasics {
		moduleBasics = append(moduleBasics, mbasic)
	}

	cosmosConfig := valcfg.BBNConfigToCosmosProviderConfig(cfg)

	cosmosConfig.Modules = moduleBasics

	provider, err := cosmosConfig.NewProvider(
		zapLogger,
		homedir,
		true,
		"babylon",
	)

	if err != nil {
		return nil, err
	}

	cp := provider.(*cosmos.CosmosProvider)

	cp.PCfg.KeyDirectory = cfg.KeyDirectory
	// Need to override this manually as otherwise oprion from config is ignored
	cp.Cdc = cosmos.MakeCodec(moduleBasics, []string{})

	err = cp.Init(context.Background())

	if err != nil {
		return nil, err
	}

	return &BabylonController{
		cp,
		logger,
		cfg.Timeout,
	}, nil
}

func (bc *BabylonController) MustGetTxSigner() string {
	address, err := bc.provider.Address()
	if err != nil {
		panic(err)
	}
	return address
}

func (bc *BabylonController) GetStakingParams() (*StakingParams, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	queryCkptClient := btcctypes.NewQueryClient(bc.provider)

	ckptQueryRequest := &btcctypes.QueryParamsRequest{}
	ckptParamRes, err := queryCkptClient.Params(ctx, ckptQueryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query params of the btccheckpoint module: %v", err)
	}

	queryStakingClient := btcstakingtypes.NewQueryClient(bc.provider)
	stakingQueryRequest := &btcstakingtypes.QueryParamsRequest{}
	stakingParamRes, err := queryStakingClient.Params(ctx, stakingQueryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query staking params: %v", err)
	}
	juryPk, err := stakingParamRes.Params.JuryPk.ToBTCPK()
	if err != nil {
		return nil, err
	}

	return &StakingParams{
		ComfirmationTimeBlocks:    ckptParamRes.Params.BtcConfirmationDepth,
		FinalizationTimeoutBlocks: ckptParamRes.Params.CheckpointFinalizationTimeout,
		MinSlashingTxFeeSat:       btcutil.Amount(stakingParamRes.Params.MinSlashingTxFeeSat),
		JuryPk:                    juryPk,
		SlashingAddress:           stakingParamRes.Params.SlashingAddress,
		MinComissionRate:          stakingParamRes.Params.MinCommissionRate,
	}, nil
}

func (bc *BabylonController) reliablySendMsg(msg sdk.Msg) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg})
}

func (bc *BabylonController) reliablySendMsgs(msgs []sdk.Msg) (*provider.RelayerTxResponse, error) {
	var (
		rlyResp     *provider.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		rlyResp = rtr
		callbackErr = err
		wg.Done()
	}

	wg.Add(1)

	// convert message type
	relayerMsgs := []pv.RelayerMessage{}
	for _, m := range msgs {
		relayerMsg := cosmos.NewCosmosMessage(m, func(signer string) {})
		relayerMsgs = append(relayerMsgs, relayerMsg)
	}

	ctx := context.Background()
	if err := retry.Do(func() error {
		var sendMsgErr error
		krErr := bc.accessKeyWithLock(func() {
			sendMsgErr = bc.provider.SendMessagesToMempool(ctx, relayerMsgs, "", ctx, []func(*provider.RelayerTxResponse, error){callback})
		})
		if krErr != nil {
			bc.logger.WithFields(logrus.Fields{
				"error": krErr,
			}).Error("unrecoverable err when submitting the tx, skip retrying")
			return retry.Unrecoverable(krErr)
		}
		if sendMsgErr != nil {
			if IsUnrecoverable(sendMsgErr) {
				bc.logger.WithFields(logrus.Fields{
					"error": sendMsgErr,
				}).Error("unrecoverable err when submitting the tx, skip retrying")
				return retry.Unrecoverable(sendMsgErr)
			}
			if IsExpected(sendMsgErr) {
				bc.logger.WithFields(logrus.Fields{
					"error": sendMsgErr,
				}).Error("expected err when submitting the tx, skip retrying")
				return nil
			}
			return sendMsgErr
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		bc.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": rtyAttNum,
			"error":        err,
		}).Error()
	})); err != nil {
		return nil, err
	}

	wg.Wait()

	if callbackErr != nil {
		if IsExpected(callbackErr) {
			return nil, nil
		}
		return nil, callbackErr
	}

	if rlyResp == nil {
		// this case could happen if the error within the retry is an expected error
		return nil, nil
	}

	if rlyResp.Code != 0 {
		return rlyResp, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, nil
}

// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterValidator(
	bbnPubKey *secp256k1.PubKey,
	btcPubKey *bbntypes.BIP340PubKey,
	pop *btcstakingtypes.ProofOfPossession,
	commission sdkTypes.Dec,
) (*provider.RelayerTxResponse, error) {

	// TODO: This should be user configurable
	emptyDesc := sttypes.Description{}

	msg := &btcstakingtypes.MsgCreateBTCValidator{
		Signer:      bc.MustGetTxSigner(),
		BabylonPk:   bbnPubKey,
		BtcPk:       btcPubKey,
		Pop:         pop,
		Commission:  &commission,
		Description: &emptyDesc,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(btcPubKey *bbntypes.BIP340PubKey, startHeight uint64, pubRandList []bbntypes.SchnorrPubRand, sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error) {
	msg := &finalitytypes.MsgCommitPubRandList{
		Signer:      bc.MustGetTxSigner(),
		ValBtcPk:    btcPubKey,
		StartHeight: startHeight,
		PubRandList: pubRandList,
		Sig:         sig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJurySig(btcPubKey *bbntypes.BIP340PubKey, delPubKey *bbntypes.BIP340PubKey, stakingTxHash string, sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgAddJurySig{
		Signer:        bc.MustGetTxSigner(),
		ValPk:         btcPubKey,
		DelPk:         delPubKey,
		StakingTxHash: stakingTxHash,
		Sig:           sig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SubmitJuryUnbondingSigs submits the Jury signatures via a MsgAddJuryUnbondingSigs to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJuryUnbondingSigs(
	btcPubKey *bbntypes.BIP340PubKey,
	delPubKey *bbntypes.BIP340PubKey,
	stakingTxHash string,
	unbondingSig *bbntypes.BIP340Signature,
	slashUnbondingSig *bbntypes.BIP340Signature,
) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgAddJuryUnbondingSigs{
		Signer:                 bc.MustGetTxSigner(),
		ValPk:                  btcPubKey,
		DelPk:                  delPubKey,
		StakingTxHash:          stakingTxHash,
		UnbondingTxSig:         unbondingSig,
		SlashingUnbondingTxSig: slashUnbondingSig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(btcPubKey *bbntypes.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *bbntypes.SchnorrEOTSSig) (*provider.RelayerTxResponse, error) {
	msg := &finalitytypes.MsgAddFinalitySig{
		Signer:              bc.MustGetTxSigner(),
		ValBtcPk:            btcPubKey,
		BlockHeight:         blockHeight,
		BlockLastCommitHash: blockHash,
		FinalitySig:         sig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (bc *BabylonController) SubmitBatchFinalitySigs(btcPubKey *bbntypes.BIP340PubKey, blocks []*types.BlockInfo, sigs []*bbntypes.SchnorrEOTSSig) (*provider.RelayerTxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, b := range blocks {
		msg := &finalitytypes.MsgAddFinalitySig{
			Signer:              bc.MustGetTxSigner(),
			ValBtcPk:            btcPubKey,
			BlockHeight:         b.Height,
			BlockLastCommitHash: b.LastCommitHash,
			FinalitySig:         sigs[i],
		}
		msgs = append(msgs, msg)
	}

	res, err := bc.reliablySendMsgs(msgs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (bc *BabylonController) SubmitValidatorUnbondingSig(
	valPubKey *bbntypes.BIP340PubKey,
	delPubKey *bbntypes.BIP340PubKey,
	stakingTxHash string,
	sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error) {

	msg := &btcstakingtypes.MsgAddValidatorUnbondingSig{
		Signer:         bc.MustGetTxSigner(),
		ValPk:          valPubKey,
		DelPk:          delPubKey,
		StakingTxHash:  stakingTxHash,
		UnbondingTxSig: sig,
	}

	res, err := bc.reliablySendMsg(msg)

	if err != nil {
		return nil, err
	}

	bc.logger.WithFields(logrus.Fields{
		"validator": valPubKey.MarshalHex(),
		"code":      res.Code,
		"height":    res.Height,
		"tx_hash":   res.TxHash,
	}).Debug("Succesfuly submitted validator signature for unbonding tx")

	return res, nil
}

// Currently this is only used for e2e tests, probably does not need to add it into the interface
func (bc *BabylonController) CreateBTCDelegation(
	delBabylonPk *secp256k1.PubKey,
	pop *btcstakingtypes.ProofOfPossession,
	stakingTx *btcstakingtypes.BabylonBTCTaprootTx,
	stakingTxInfo *btcctypes.TransactionInfo,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSig *bbntypes.BIP340Signature,
) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgCreateBTCDelegation{
		Signer:        bc.MustGetTxSigner(),
		BabylonPk:     delBabylonPk,
		Pop:           pop,
		StakingTx:     stakingTx,
		StakingTxInfo: stakingTxInfo,
		SlashingTx:    slashingTx,
		DelegatorSig:  delSig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	bc.logger.Infof("successfully submitted a BTC delegation, code: %v, height: %v, tx hash: %s", res.Code, res.Height, res.TxHash)
	return res, nil
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) CreateBTCUndelegation(
	unbondingTx *btcstakingtypes.BabylonBTCTaprootTx,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSig *bbntypes.BIP340Signature,
) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgBTCUndelegate{
		Signer:               bc.MustGetTxSigner(),
		UnbondingTx:          unbondingTx,
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
func (bc *BabylonController) InsertBtcBlockHeaders(headers []*bbntypes.BTCHeaderBytes) (*provider.RelayerTxResponse, error) {
	msgs := make([]sdk.Msg, 0, len(headers))
	for _, h := range headers {
		msg := &btclctypes.MsgInsertHeader{
			Signer: bc.MustGetTxSigner(),
			Header: h,
		}
		msgs = append(msgs, msg)
	}

	res, err := bc.reliablySendMsgs(msgs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Note: the following queries are only for PoC
// QueryHeightWithLastPubRand queries the height of the last block with public randomness
func (bc *BabylonController) QueryHeightWithLastPubRand(btcPubKey *bbntypes.BIP340PubKey) (uint64, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	// query the last committed public randomness
	queryRequest := &finalitytypes.QueryListPublicRandomnessRequest{
		ValBtcPkHex: btcPubKey.MarshalHex(),
		Pagination: &sdkquery.PageRequest{
			Limit:   1,
			Reverse: true,
		},
	}

	res, err := queryClient.ListPublicRandomness(ctx, queryRequest)
	if err != nil {
		return 0, err
	}

	if len(res.PubRandMap) == 0 {
		return 0, nil
	}

	ks := maps.Keys(res.PubRandMap)
	if len(ks) > 1 {
		return 0, fmt.Errorf("the query should not return more than one public rand item")
	}

	return ks[0], nil
}

// QueryPendingBTCDelegations queries BTC delegations that need a Jury sig
// it is only used when the program is running in Jury mode
func (bc *BabylonController) QueryPendingBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &btcstakingtypes.QueryPendingBTCDelegationsRequest{}
	res, err := queryClient.PendingBTCDelegations(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	return res.BtcDelegations, nil
}

// QueryUnbondindBTCDelegations queries BTC delegations that need a Jury sig for unbodning
// it is only used when the program is running in Jury mode
func (bc *BabylonController) QueryUnbondindBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &btcstakingtypes.QueryUnbondingBTCDelegationsRequest{}
	res, err := queryClient.UnbondingBTCDelegations(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	return res.BtcDelegations, nil
}

// QueryValidators queries BTC validators
func (bc *BabylonController) QueryValidators() ([]*btcstakingtypes.BTCValidator, error) {
	var validators []*btcstakingtypes.BTCValidator
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

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

func (bc *BabylonController) QueryBtcLightClientTip() (*btclctypes.BTCHeaderInfo, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btclctypes.NewQueryClient(clientCtx)

	queryRequest := &btclctypes.QueryTipRequest{}
	res, err := queryClient.Tip(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC tip: %v", err)
	}

	return res.Header, nil
}

func (bc *BabylonController) getNDelegations(
	valBtcPk *bbntypes.BIP340PubKey,
	startKey []byte,
	n uint64,
) ([]*btcstakingtypes.BTCDelegation, []byte, error) {
	pagination := &sdkquery.PageRequest{
		Key:   startKey,
		Limit: n,
	}

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	queryRequest := &btcstakingtypes.QueryBTCValidatorDelegationsRequest{
		ValBtcPkHex: valBtcPk.MarshalHex(),
		Pagination:  pagination,
	}
	res, err := queryClient.BTCValidatorDelegations(ctx, queryRequest)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	var delegations []*btcstakingtypes.BTCDelegation

	for _, dels := range res.BtcDelegatorDelegations {
		delegations = append(delegations, dels.Dels...)
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
	match func(*btcstakingtypes.BTCDelegation) bool,
) ([]*btcstakingtypes.BTCDelegation, error) {
	batchSize := 100
	var delegations []*btcstakingtypes.BTCDelegation
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

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryBTCValidatorDelegations(valBtcPk *bbntypes.BIP340PubKey, max uint64) ([]*btcstakingtypes.BTCDelegation, error) {
	return bc.getNValidatorDelegationsMatchingCriteria(
		valBtcPk,
		max,
		// fitlering function which always returns true as we want all delegations
		func(*btcstakingtypes.BTCDelegation) bool { return true },
	)
}

func (bc *BabylonController) QueryBTCValidatorUnbondingDelegations(valBtcPk *bbntypes.BIP340PubKey, max uint64) ([]*btcstakingtypes.BTCDelegation, error) {
	// TODO Check what is the order of returned delegations. Ideally we would return
	// delegation here from the first one which received undelegation
	return bc.getNValidatorDelegationsMatchingCriteria(
		valBtcPk,
		max,
		func(del *btcstakingtypes.BTCDelegation) bool {
			return del.BtcUndelegation != nil && del.BtcUndelegation.ValidatorUnbondingSig == nil
		},
	)
}

// QueryValidatorVotingPower queries the voting power of the validator at a given height
func (bc *BabylonController) QueryValidatorVotingPower(btcPubKey *bbntypes.BIP340PubKey, blockHeight uint64) (uint64, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &btcstakingtypes.QueryBTCValidatorPowerAtHeightRequest{
		ValBtcPkHex: btcPubKey.MarshalHex(),
		Height:      blockHeight,
	}
	res, err := queryClient.BTCValidatorPowerAtHeight(ctx, queryRequest)
	if err != nil {
		return 0, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.VotingPower, nil
}

// QueryBlockFinalization queries whether the block has been finalized
func (bc *BabylonController) QueryBlockFinalization(blockHeight uint64) (bool, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	// query the indexed block at the given height
	queryRequest := &finalitytypes.QueryBlockRequest{
		Height: blockHeight,
	}
	res, err := queryClient.Block(ctx, queryRequest)
	if err != nil {
		return false, fmt.Errorf("failed to query indexed block at height %v: %w", blockHeight, err)
	}

	return res.Block.Finalized, nil
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

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	for {
		queryRequest := &finalitytypes.QueryListBlocksRequest{
			Status:     status,
			Pagination: pagination,
		}
		res, err := queryClient.ListBlocks(ctx, queryRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to query finalized blocks: %v", err)
		}
		for _, b := range res.Blocks {
			ib := &types.BlockInfo{
				Height:         b.Height,
				LastCommitHash: b.LastCommitHash,
			}
			blocks = append(blocks, ib)
		}
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return blocks, nil
}

func getContextWithCancel(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel
}

func (bc *BabylonController) QueryHeader(height int64) (*ctypes.ResultHeader, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	headerResp, err := bc.provider.RPCClient.Header(ctx, &height)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return headerResp, nil
}

func (bc *BabylonController) QueryBestHeader() (*ctypes.ResultHeader, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := bc.provider.RPCClient.BlockchainInfo(ctx, 0, 0)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return &ctypes.ResultHeader{
		Header: &chainInfo.BlockMetas[0].Header,
	}, nil
}

func (bc *BabylonController) QueryNodeStatus() (*ctypes.ResultStatus, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	status, err := bc.provider.QueryStatus(ctx)
	if err != nil {
		return nil, err
	}
	return status, nil
}

// accessKeyWithLock triggers a function that access key ring while acquiring
// the file system lock, in order to remain thread-safe when multiple concurrent
// relayers are running on the same machine and accessing the same keyring
// adapted from
// https://github.com/babylonchain/babylon-relayer/blob/f962d0940832a8f84f747c5d9cbc67bc1b156386/bbnrelayer/utils.go#L212
func (bc *BabylonController) accessKeyWithLock(accessFunc func()) error {
	// use lock file to guard concurrent access to the keyring
	lockFilePath := path.Join(bc.provider.PCfg.KeyDirectory, "keys.lock")
	lock := fslock.New(lockFilePath)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire file system lock (%s): %w", lockFilePath, err)
	}

	// trigger function that access keyring
	accessFunc()

	// unlock and release access
	if err := lock.Unlock(); err != nil {
		return fmt.Errorf("error unlocking file system lock (%s), please manually delete", lockFilePath)
	}

	return nil
}

func (bc *BabylonController) Close() error {
	if !bc.provider.RPCClient.IsRunning() {
		return nil
	}

	return bc.provider.RPCClient.Stop()
}
