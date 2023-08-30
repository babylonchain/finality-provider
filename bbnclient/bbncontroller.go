package babylonclient

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	bbnapp "github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcutil"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	pv "github.com/cosmos/relayer/v2/relayer/provider"
	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"

	"github.com/babylonchain/btc-validator/valcfg"
)

var _ BabylonClient = &BabylonController{}

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
		err := bc.provider.SendMessagesToMempool(ctx, relayerMsgs, "", ctx, []func(*provider.RelayerTxResponse, error){callback})
		if err != nil {
			if !IsRetriable(err) {
				bc.logger.WithFields(logrus.Fields{
					"error": err,
				}).Error("unrecoverable err when submitting the tx, skip retrying")
				return retry.Unrecoverable(err)
			}
			if IsExpected(err) {
				bc.logger.WithFields(logrus.Fields{
					"error": err,
				}).Error("expected err when submitting the tx, skip retrying")
				return nil
			}
			return err
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
		return nil, callbackErr
	}

	if rlyResp.Code != 0 {
		return rlyResp, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, nil
}

// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *btcstakingtypes.ProofOfPossession) (*provider.RelayerTxResponse, error) {
	msg := &btcstakingtypes.MsgCreateBTCValidator{
		Signer:    bc.MustGetTxSigner(),
		BabylonPk: bbnPubKey,
		BtcPk:     btcPubKey,
		Pop:       pop,
	}

	res, err := bc.reliablySendMsg(msg)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) (*provider.RelayerTxResponse, error) {
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
func (bc *BabylonController) SubmitJurySig(btcPubKey *types.BIP340PubKey, delPubKey *types.BIP340PubKey, stakingTxHash string, sig *types.BIP340Signature) (*provider.RelayerTxResponse, error) {
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

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) (*provider.RelayerTxResponse, error) {
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

// Currently this is only used for e2e tests, probably does not need to add it into the interface
func (bc *BabylonController) CreateBTCDelegation(
	delBabylonPk *secp256k1.PubKey,
	pop *btcstakingtypes.ProofOfPossession,
	stakingTx *btcstakingtypes.StakingTx,
	stakingTxInfo *btcctypes.TransactionInfo,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSig *types.BIP340Signature,
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

// Insert BTC block header using rpc client
// Currently this is only used for e2e tests, probably does not need to add it into the interface
func (bc *BabylonController) InsertBtcBlockHeaders(headers []*types.BTCHeaderBytes) (*provider.RelayerTxResponse, error) {
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
func (bc *BabylonController) QueryHeightWithLastPubRand(btcPubKey *types.BIP340PubKey) (uint64, error) {
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
	var delegations []*btcstakingtypes.BTCDelegation

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
	delegations = append(delegations, res.BtcDelegations...)

	return delegations, nil
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

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryLatestFinalisedBlocks(count uint64) ([]*finalitytypes.IndexedBlock, error) {
	var blocks []*finalitytypes.IndexedBlock
	pagination := &sdkquery.PageRequest{
		Limit:   count,
		Reverse: true,
	}

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := finalitytypes.NewQueryClient(clientCtx)

	for {
		queryRequest := &finalitytypes.QueryListBlocksRequest{
			Status:     finalitytypes.QueriedBlockStatus_FINALIZED,
			Pagination: pagination,
		}
		res, err := queryClient.ListBlocks(ctx, queryRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to query finalized blocks: %v", err)
		}
		blocks = append(blocks, res.Blocks...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return blocks, nil
}

// Currently this is only used for e2e tests, probably does not need to add this into the interface
func (bc *BabylonController) QueryBTCValidatorDelegations(valBtcPk *types.BIP340PubKey) ([]*btcstakingtypes.BTCDelegation, error) {
	var delegations []*btcstakingtypes.BTCDelegation
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	ctx, cancel := getContextWithCancel(bc.timeout)
	defer cancel()

	clientCtx := sdkclient.Context{Client: bc.provider.RPCClient}

	queryClient := btcstakingtypes.NewQueryClient(clientCtx)

	for {
		queryRequest := &btcstakingtypes.QueryBTCValidatorDelegationsRequest{
			ValBtcPkHex: valBtcPk.MarshalHex(),
			Pagination:  pagination,
		}
		res, err := queryClient.BTCValidatorDelegations(ctx, queryRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
		}
		for _, dels := range res.BtcDelegatorDelegations {
			delegations = append(delegations, dels.Dels...)
		}
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return delegations, nil
}

// QueryValidatorVotingPower queries the voting power of the validator at a given height
func (bc *BabylonController) QueryValidatorVotingPower(btcPubKey *types.BIP340PubKey, blockHeight uint64) (uint64, error) {
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

// QueryIndexedBlock queries the Babylon indexed block with the given height
func (bc *BabylonController) QueryIndexedBlock(blockHeight uint64) (*finalitytypes.IndexedBlock, error) {
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
		return nil, fmt.Errorf("failed to query indexed block at height %v: %w", blockHeight, err)
	}

	return res.Block, nil
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

func (bc *BabylonController) Close() error {
	if !bc.provider.RPCClient.IsRunning() {
		return nil
	}

	return bc.provider.RPCClient.Stop()
}
