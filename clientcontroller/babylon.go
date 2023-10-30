package clientcontroller

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"cosmossdk.io/math"
	"github.com/avast/retry-go/v4"
	bbnapp "github.com/babylonchain/babylon/app"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	"sigs.k8s.io/yaml"

	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

var _ ClientController = &BabylonController{}

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint64
	// W-deep
	FinalizationTimeoutBlocks uint64

	// Minimum amount of satoshis required for slashing transaction
	MinSlashingTxFeeSat btcutil.Amount

	// Bitcoin public key of the current jury
	JuryPk *btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingAddress string

	// Minimum commission required by the consumer chain
	MinCommissionRate string
}

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
		MinCommissionRate:         stakingParamRes.Params.MinCommissionRate.String(),
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
			sendMsgErr = ConvertErrType(sendMsgErr)
			if IsUnrecoverable(sendMsgErr) {
				bc.logger.WithFields(logrus.Fields{
					"error": sendMsgErr,
				}).Error("unrecoverable err when submitting the tx, skip retrying")
				return retry.Unrecoverable(sendMsgErr)
			}
			if IsExpected(sendMsgErr) {
				// this is necessary because if err is returned
				// the callback function will not be executed so
				// that the inside wg.Done will not be executed
				wg.Done()
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
		callbackErr = ConvertErrType(callbackErr)
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
	chainPk []byte,
	valPk *btcec.PublicKey,
	pop []byte,
	commission string,
	description string,
) (*types.TxResponse, error) {
	var bbnPop btcstakingtypes.ProofOfPossession
	if err := bbnPop.Unmarshal(pop); err != nil {
		return nil, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	sdkCommission, err := math.LegacyNewDecFromStr(commission)
	if err != nil {
		return nil, fmt.Errorf("invalid commission: %w", err)
	}

	var sdkDescription sttypes.Description
	if err := yaml.Unmarshal([]byte(description), &sdkDescription); err != nil {
		return nil, fmt.Errorf("invalid descirption: %w", err)
	}

	msg := &btcstakingtypes.MsgCreateBTCValidator{
		Signer:      bc.MustGetTxSigner(),
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
func (bc *BabylonController) CommitPubRandList(valPk *btcec.PublicKey, startHeight uint64, pubRandList [][]byte, sig *schnorr.Signature) (*types.TxResponse, error) {
	schnorrPubRandList := make([]bbntypes.SchnorrPubRand, 0, len(pubRandList))
	for _, r := range pubRandList {
		schnorrPubRand, err := bbntypes.NewSchnorrPubRand(r)
		if err != nil {
			return nil, fmt.Errorf("invalid public randomness: %w", err)
		}
		schnorrPubRandList = append(schnorrPubRandList, *schnorrPubRand)
	}

	bip340Sig := bbntypes.NewBIP340SignatureFromBTCSig(sig)

	msg := &finalitytypes.MsgCommitPubRandList{
		Signer:      bc.MustGetTxSigner(),
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

// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJurySig(valPk *btcec.PublicKey, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error) {
	bip340Sig := bbntypes.NewBIP340SignatureFromBTCSig(sig)

	msg := &btcstakingtypes.MsgAddJurySig{
		Signer:        bc.MustGetTxSigner(),
		ValPk:         bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		DelPk:         bbntypes.NewBIP340PubKeyFromBTCPK(delPk),
		StakingTxHash: stakingTxHash,
		Sig:           &bip340Sig,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

// SubmitJuryUnbondingSigs submits the Jury signatures via a MsgAddJuryUnbondingSigs to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJuryUnbondingSigs(
	valPk *btcec.PublicKey,
	delPk *btcec.PublicKey,
	stakingTxHash string,
	unbondingSig *schnorr.Signature,
	slashUnbondingSig *schnorr.Signature,
) (*types.TxResponse, error) {
	bip340UnbondingSig := bbntypes.NewBIP340SignatureFromBTCSig(unbondingSig)

	bip340SlashUnbondingSig := bbntypes.NewBIP340SignatureFromBTCSig(slashUnbondingSig)

	msg := &btcstakingtypes.MsgAddJuryUnbondingSigs{
		Signer:                 bc.MustGetTxSigner(),
		ValPk:                  bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		DelPk:                  bbntypes.NewBIP340PubKeyFromBTCPK(delPk),
		StakingTxHash:          stakingTxHash,
		UnbondingTxSig:         &bip340UnbondingSig,
		SlashingUnbondingTxSig: &bip340SlashUnbondingSig,
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
		Signer:              bc.MustGetTxSigner(),
		ValBtcPk:            bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		BlockHeight:         blockHeight,
		BlockLastCommitHash: blockHash,
		FinalitySig:         bbntypes.NewSchnorrEOTSSigFromModNScalar(sig),
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
			Signer:              bc.MustGetTxSigner(),
			ValBtcPk:            bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
			BlockHeight:         b.Height,
			BlockLastCommitHash: b.LastCommitHash,
			FinalitySig:         bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]),
		}
		msgs = append(msgs, msg)
	}

	res, err := bc.reliablySendMsgs(msgs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
}

func (bc *BabylonController) SubmitValidatorUnbondingSig(
	valPk *btcec.PublicKey,
	delPk *btcec.PublicKey,
	stakingTxHash string,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	valBtcPk := bbntypes.NewBIP340PubKeyFromBTCPK(valPk)

	bip340Sig := bbntypes.NewBIP340SignatureFromBTCSig(sig)

	msg := &btcstakingtypes.MsgAddValidatorUnbondingSig{
		Signer:         bc.MustGetTxSigner(),
		ValPk:          valBtcPk,
		DelPk:          bbntypes.NewBIP340PubKeyFromBTCPK(delPk),
		StakingTxHash:  stakingTxHash,
		UnbondingTxSig: &bip340Sig,
	}

	res, err := bc.reliablySendMsg(msg)

	if err != nil {
		return nil, err
	}

	bc.logger.WithFields(logrus.Fields{
		"validator": valBtcPk.MarshalHex(),
		"code":      res.Code,
		"height":    res.Height,
		"tx_hash":   res.TxHash,
	}).Debug("Succesfuly submitted validator signature for unbonding tx")

	return &types.TxResponse{TxHash: res.TxHash, Events: res.Events}, nil
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

// QueryBTCDelegations queries BTC delegations that need a Jury signature
// with the given status (either pending or unbonding)
// it is only used when the program is running in Jury mode
func (bc *BabylonController) QueryBTCDelegations(status types.DelegationStatus, limit uint64) ([]*types.Delegation, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &btcstakingtypes.QueryBTCDelegationsRequest{
		Status:     btcstakingtypes.BTCDelegationStatus(status),
		Pagination: pagination,
	}
	res, err := queryClient.BTCDelegations(ctx, queryRequest)
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
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

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

// QueryValidators queries BTC validators
// Currently this is only used for e2e tests, probably does not need to add this into the interface
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
) ([]*types.Delegation, []byte, error) {
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

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryBTCValidatorDelegations(valBtcPk *bbntypes.BIP340PubKey, max uint64) ([]*types.Delegation, error) {
	return bc.getNValidatorDelegationsMatchingCriteria(
		valBtcPk,
		max,
		// fitlering function which always returns true as we want all delegations
		func(*types.Delegation) bool { return true },
	)
}

func (bc *BabylonController) QueryVotesAtHeight(height uint64) ([]bbntypes.BIP340PubKey, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

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

func (bc *BabylonController) QueryBTCValidatorUnbondingDelegations(valPk *btcec.PublicKey, max uint64) ([]*types.Delegation, error) {
	// TODO Check what is the order of returned delegations. Ideally we would return
	// delegation here from the first one which received undelegation

	return bc.getNValidatorDelegationsMatchingCriteria(
		bbntypes.NewBIP340PubKeyFromBTCPK(valPk),
		max,
		func(del *types.Delegation) bool {
			return del.BtcUndelegation != nil && del.BtcUndelegation.ValidatorUnbondingSig == nil
		},
	)
}

// QueryValidatorVotingPower queries the voting power of the validator at a given height
func (bc *BabylonController) QueryValidatorVotingPower(valPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	// query all the unsigned delegations
	queryRequest := &btcstakingtypes.QueryBTCValidatorPowerAtHeightRequest{
		ValBtcPkHex: bbntypes.NewBIP340PubKeyFromBTCPK(valPk).MarshalHex(),
		Height:      blockHeight,
	}
	res, err := queryClient.BTCValidatorPowerAtHeight(ctx, queryRequest)
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

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

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

	return blocks, nil
}

func getContextWithCancel(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel
}

func (bc *BabylonController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

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
		Height:         height,
		LastCommitHash: res.Block.LastCommitHash,
		Finalized:      res.Block.Finalized,
	}, nil
}

func (bc *BabylonController) QueryActivatedHeight() (uint64, error) {
	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

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
	ctx, cancel := getContextWithCancel(bc.timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := bc.provider.RPCClient.BlockchainInfo(ctx, 0, 0)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return &types.BlockInfo{
		Height:         uint64(chainInfo.BlockMetas[0].Header.Height),
		LastCommitHash: chainInfo.BlockMetas[0].Header.LastCommitHash,
	}, nil
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

func ConvertErrType(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case strings.Contains(err.Error(), btcstakingtypes.ErrBTCValAlreadySlashed.Error()):
		return types.ErrValidatorSlashed
	case strings.Contains(err.Error(), finalitytypes.ErrBlockNotFound.Error()):
		return types.ErrBlockNotFound
	case strings.Contains(err.Error(), finalitytypes.ErrInvalidFinalitySig.Error()):
		return types.ErrInvalidFinalitySig
	case strings.Contains(err.Error(), finalitytypes.ErrHeightTooHigh.Error()):
		return types.ErrHeightTooHigh
	case strings.Contains(err.Error(), finalitytypes.ErrNoPubRandYet.Error()):
		return types.ErrNoPubRandYet
	case strings.Contains(err.Error(), finalitytypes.ErrPubRandNotFound.Error()):
		return types.ErrPubRandNotFound
	case strings.Contains(err.Error(), finalitytypes.ErrTooFewPubRand.Error()):
		return types.ErrTooFewPubRand
	case strings.Contains(err.Error(), finalitytypes.ErrInvalidPubRand.Error()):
		return types.ErrInvalidPubRand
	case strings.Contains(err.Error(), finalitytypes.ErrDuplicatedFinalitySig.Error()):
		return types.ErrDuplicatedFinalitySig
	default:
		return err
	}
}

func ConvertDelegationType(del *btcstakingtypes.BTCDelegation) *types.Delegation {
	var (
		jurySchnorrSig *schnorr.Signature
		err            error
	)
	if del.JurySig != nil {
		jurySchnorrSig, err = del.JurySig.ToBTCSig()
		if err != nil {
			panic(err)
		}
	}
	return &types.Delegation{
		BtcPk:           del.BtcPk.MustToBTCPK(),
		ValBtcPk:        del.ValBtcPk.MustToBTCPK(),
		StartHeight:     del.StartHeight,
		EndHeight:       del.EndHeight,
		TotalSat:        del.TotalSat,
		StakingTx:       del.StakingTx,
		SlashingTx:      del.SlashingTx,
		JurySig:         jurySchnorrSig,
		BtcUndelegation: del.BtcUndelegation,
	}
}
