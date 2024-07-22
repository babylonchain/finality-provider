//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon-finality-gadget/sdk/btcclient"
	sdkclient "github.com/babylonchain/babylon-finality-gadget/sdk/client"
	sdkcfg "github.com/babylonchain/babylon-finality-gadget/sdk/config"
	bbncfg "github.com/babylonchain/babylon/client/config"
	bbntypes "github.com/babylonchain/babylon/types"
	api "github.com/babylonchain/finality-provider/clientcontroller/api"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	opcc "github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	base_test_manager "github.com/babylonchain/finality-provider/itest/test-manager"
	"github.com/babylonchain/finality-provider/metrics"
	"github.com/babylonchain/finality-provider/testutil/log"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	ope2e "github.com/ethereum-optimism/optimism/op-e2e"
	optestlog "github.com/ethereum-optimism/optimism/op-service/testlog"
	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	opFinalityGadgetContractPath = "../bytecode/op_finality_gadget_42eb9bf.wasm"
	consumerChainIdPrefix        = "op-stack-l2-"
	bbnAddrTopUpAmount           = 1000000
)

type BaseTestManager = base_test_manager.BaseTestManager

type OpL2ConsumerTestManager struct {
	BaseTestManager
	BabylonHandler    *e2eutils.BabylonNodeHandler
	BabylonFpApp      *service.FinalityProviderApp
	ConsumerFpApps    []*service.FinalityProviderApp
	EOTSServerHandler *e2eutils.EOTSServerHandler
	BaseDir           string
	SdkClient         *sdkclient.SdkClient
	OpSystem          *ope2e.System
}

func StartOpL2ConsumerManager(t *testing.T, numOfConsumerFPs uint8) *OpL2ConsumerTestManager {
	// Setup base dir and logger
	testDir, err := e2eutils.BaseDir("fpe2etest")
	require.NoError(t, err)

	logger := createLogger(t, zapcore.DebugLevel)

	// generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2eutils.GenerateCovenantCommittee(numCovenants, t)

	// start Babylon node
	bh := e2eutils.NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)

	// deploy op-finality-gadget contract and start op stack system
	opL2ConsumerConfig, opSys := startExtSystemsAndCreateConsumerCfg(t, logger, bh)
	// TODO: this is a hack to try to fix a flaky data race
	// https://github.com/babylonchain/finality-provider/issues/528
	time.Sleep(5 * time.Second)

	// init SDK client
	sdkClient := initSdkClient(opSys, opL2ConsumerConfig, t)

	// start multiple FPs. each FP has its own EOTS manager and finality provider app
	// there is one Babylon FP and multiple Consumer FPs
	babylonFpApp, consumerFpApps, eotsHandler := createMultiFps(
		testDir,
		bh,
		opL2ConsumerConfig,
		numOfConsumerFPs+1,
		logger,
		t,
	)

	// register consumer to Babylon (only one of the FPs needs to do it)
	opConsumerId := getConsumerChainId(&opSys.Cfg)
	babylonClient := consumerFpApps[0].GetBabylonController().(*bbncc.BabylonController)
	_, err = babylonClient.RegisterConsumerChain(
		opConsumerId,
		"OP consumer chain (test)",
		"some description about the chain",
	)
	require.NoError(t, err)
	t.Logf(log.Prefix("Register consumer %s to Babylon"), opConsumerId)

	ctm := &OpL2ConsumerTestManager{
		BaseTestManager: BaseTestManager{
			BBNClient:        babylonClient,
			CovenantPrivKeys: covenantPrivKeys,
		},
		BabylonHandler:    bh,
		EOTSServerHandler: eotsHandler,
		BabylonFpApp:      babylonFpApp,
		ConsumerFpApps:    consumerFpApps,
		BaseDir:           testDir,
		SdkClient:         sdkClient,
		OpSystem:          opSys,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func createMultiFps(
	testDir string,
	bh *e2eutils.BabylonNodeHandler,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	numOfConsumerFPs uint8,
	logger *zap.Logger,
	t *testing.T,
) (*service.FinalityProviderApp, []*service.FinalityProviderApp, *e2eutils.EOTSServerHandler) {
	babylonFpCfg, consumerFpCfgs := createFpConfigs(
		testDir,
		bh,
		opL2ConsumerConfig,
		numOfConsumerFPs,
		logger,
		t,
	)

	eotsHandler, eotsClients := startEotsManagers(testDir, t, babylonFpCfg, consumerFpCfgs, logger)

	babylonFpApp, consumerFpApps := createFpApps(
		babylonFpCfg,
		consumerFpCfgs,
		eotsClients,
		logger,
		t,
	)

	return babylonFpApp, consumerFpApps, eotsHandler
}

func createFpApps(
	babylonFpCfg *fpcfg.Config,
	consumerFpCfgs []*fpcfg.Config,
	eotsClients []*client.EOTSManagerGRpcClient,
	logger *zap.Logger,
	t *testing.T,
) (*service.FinalityProviderApp, []*service.FinalityProviderApp) {
	babylonFpApp := createBabylonFpApp(babylonFpCfg, eotsClients[0], logger, t)
	consumerFpApps := createConsumerFpApps(consumerFpCfgs, eotsClients[1:], logger, t)
	return babylonFpApp, consumerFpApps
}

func createBabylonFpApp(
	cfg *fpcfg.Config,
	eotsCli *client.EOTSManagerGRpcClient,
	logger *zap.Logger,
	t *testing.T,
) *service.FinalityProviderApp {
	cc, err := bbncc.NewBabylonConsumerController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	fpApp := createAndStartFpApp(cfg, cc, eotsCli, logger, t)
	t.Log(log.Prefix("Started Babylon FP App"))
	return fpApp
}

func createConsumerFpApps(
	cfgs []*fpcfg.Config,
	eotsClients []*client.EOTSManagerGRpcClient,
	logger *zap.Logger,
	t *testing.T,
) []*service.FinalityProviderApp {
	consumerFpApps := make([]*service.FinalityProviderApp, len(cfgs))
	for i, cfg := range cfgs {
		cc, err := opstackl2.NewOPStackL2ConsumerController(cfg.OPStackL2Config, logger)
		require.NoError(t, err)

		fpApp := createAndStartFpApp(cfg, cc, eotsClients[i], logger, t)
		t.Logf(log.Prefix("Started Consumer FP App %d"), i)
		consumerFpApps[i] = fpApp
	}
	return consumerFpApps
}

func createAndStartFpApp(
	cfg *fpcfg.Config,
	cc api.ConsumerController,
	eotsCli *client.EOTSManagerGRpcClient,
	logger *zap.Logger,
	t *testing.T,
) *service.FinalityProviderApp {
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)

	fpApp, err := service.NewFinalityProviderApp(cfg, bc, cc, eotsCli, fpdb, logger)
	require.NoError(t, err)

	err = fpApp.Start()
	require.NoError(t, err)
	t.Logf(log.Prefix("Started FP App"))

	return fpApp
}

// create configs for multiple FPs. the first config is a Babylon FP, the rest are OP FPs
func createFpConfigs(
	testDir string,
	bh *e2eutils.BabylonNodeHandler,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	numOfConsumerFPs uint8,
	logger *zap.Logger,
	t *testing.T,
) (*fpcfg.Config, []*fpcfg.Config) {
	babylonFpCfg := createBabylonFpConfig(testDir, bh, logger, t)
	consumerFpCfgs := createConsumerFpConfigs(
		testDir,
		bh,
		opL2ConsumerConfig,
		numOfConsumerFPs,
		logger,
		t,
	)
	return babylonFpCfg, consumerFpCfgs
}

func createBabylonFpConfig(
	testDir string,
	bh *e2eutils.BabylonNodeHandler,
	logger *zap.Logger,
	t *testing.T,
) *fpcfg.Config {
	fpHomeDir := filepath.Join(testDir, "babylon-fp-home")
	t.Logf(log.Prefix("Babylon FP home dir: %s"), fpHomeDir)

	cfg := createBaseFpConfig(fpHomeDir, 0, logger)
	cfg.BabylonConfig.KeyDirectory = filepath.Join(testDir, "babylon-fp-home-keydir")

	fpBbnKeyInfo := createChainKey(cfg.BabylonConfig, t)
	fundBBNAddr(bh, fpBbnKeyInfo, t)

	return cfg
}

func createConsumerFpConfigs(
	testDir string,
	bh *e2eutils.BabylonNodeHandler,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	numOfConsumerFPs uint8,
	logger *zap.Logger,
	t *testing.T,
) []*fpcfg.Config {
	consumerFpConfigs := make([]*fpcfg.Config, numOfConsumerFPs)

	for i := 0; i < int(numOfConsumerFPs); i++ {
		fpHomeDir := filepath.Join(testDir, fmt.Sprintf("consumer-fp-home-%d", i))
		t.Logf(log.Prefix("Consumer FP home dir: %s"), fpHomeDir)

		cfg := createBaseFpConfig(fpHomeDir, i+1, logger)
		cfg.BabylonConfig.KeyDirectory = filepath.Join(testDir, fmt.Sprintf("consumer-fp-home-keydir-%d", i))

		fpBbnKeyInfo := createChainKey(cfg.BabylonConfig, t)
		fundBBNAddr(bh, fpBbnKeyInfo, t)

		opcc := *opL2ConsumerConfig
		opcc.KeyDirectory = cfg.BabylonConfig.KeyDirectory
		opcc.Key = cfg.BabylonConfig.Key
		cfg.OPStackL2Config = &opcc

		consumerFpConfigs[i] = cfg
	}

	return consumerFpConfigs
}

func createBaseFpConfig(fpHomeDir string, index int, logger *zap.Logger) *fpcfg.Config {
	// customize ports
	// FP default RPC port is 12581, EOTS default RPC port i is 12582
	// FP default metrics port is 2112, EOTS default metrics port is 2113
	cfg := e2eutils.DefaultFpConfigWithPorts(
		"this should be the keyring dir", // this will be replaced later
		fpHomeDir,
		fpcfg.DefaultRPCPort-index,
		metrics.DefaultFpConfig().Port-index,
		eotsconfig.DefaultRPCPort+index,
	)
	cfg.LogLevel = logger.Level().String()
	cfg.StatusUpdateInterval = 2 * time.Second
	cfg.RandomnessCommitInterval = 2 * time.Second
	cfg.NumPubRand = 64
	cfg.MinRandHeightGap = 1000
	return cfg
}

func createChainKey(bbnConfig *fpcfg.BBNConfig, t *testing.T) *types.ChainKeyInfo {
	fpBbnKeyInfo, err := service.CreateChainKey(
		bbnConfig.KeyDirectory,
		bbnConfig.ChainID,
		bbnConfig.Key,
		bbnConfig.KeyringBackend,
		e2eutils.Passphrase,
		e2eutils.HdPath,
		"",
	)
	require.NoError(t, err)
	return fpBbnKeyInfo
}

func fundBBNAddr(bh *e2eutils.BabylonNodeHandler, fpBbnKeyInfo *types.ChainKeyInfo, t *testing.T) {
	err := bh.BabylonNode.TxBankSend(
		fpBbnKeyInfo.AccAddress.String(),
		fmt.Sprintf("%dubbn", bbnAddrTopUpAmount),
	)
	require.NoError(t, err)

	// check balance
	require.Eventually(t, func() bool {
		balance, err := bh.BabylonNode.CheckAddrBalance(fpBbnKeyInfo.AccAddress.String())
		if err != nil {
			t.Logf("Error checking balance: %v", err)
			return false
		}
		return balance == bbnAddrTopUpAmount
	}, 30*time.Second, 2*time.Second, fmt.Sprintf("failed to top up %s", fpBbnKeyInfo.AccAddress.String()))
	t.Logf(log.Prefix("Sent %dubbn to %s"), bbnAddrTopUpAmount, fpBbnKeyInfo.AccAddress.String())
}

func startEotsManagers(
	testDir string,
	t *testing.T,
	babylonFpCfg *fpcfg.Config,
	consumerFpCfgs []*fpcfg.Config,
	logger *zap.Logger,
) (*e2eutils.EOTSServerHandler, []*client.EOTSManagerGRpcClient) {
	allConfigs := append([]*fpcfg.Config{babylonFpCfg}, consumerFpCfgs...)
	eotsClients := make([]*client.EOTSManagerGRpcClient, 0, len(allConfigs))
	eotsHomeDirs := make([]string, 0, len(allConfigs))
	eotsConfigs := make([]*eotsconfig.Config, 0, len(allConfigs))

	// start EOTS servers
	for i := 0; i < len(allConfigs); i++ {
		eotsHomeDir := filepath.Join(testDir, fmt.Sprintf("eots-home-%d", i))
		eotsHomeDirs = append(eotsHomeDirs, eotsHomeDir)

		// customize ports
		// FP default RPC port is 12581, EOTS default RPC port i is 12582
		// FP default metrics port is 2112, EOTS default metrics port is 2113
		eotsCfg := eotsconfig.DefaultConfigWithHomePathAndPorts(
			eotsHomeDir,
			eotsconfig.DefaultRPCPort+i,
			metrics.DefaultEotsConfig().Port+i,
		)
		eotsConfigs = append(eotsConfigs, eotsCfg)
	}
	eh := e2eutils.NewEOTSServerHandlerMultiFP(t, eotsConfigs, eotsHomeDirs, logger)
	eh.Start()

	// create EOTS clients
	for i := 0; i < len(allConfigs); i++ {
		// wait for EOTS servers to start
		// see https://github.com/babylonchain/finality-provider/pull/517
		require.Eventually(t, func() bool {
			eotsCli, err := client.NewEOTSManagerGRpcClient(allConfigs[i].EOTSManagerAddress)
			if err != nil {
				t.Logf(log.Prefix("Error creating EOTS client: %v"), err)
				return false
			}
			eotsClients = append(eotsClients, eotsCli)
			return true
		}, 5*time.Second, time.Second, "Failed to create EOTS clients")
	}

	return eh, eotsClients
}

func initSdkClient(
	opSys *ope2e.System,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	t *testing.T,
) *sdkclient.SdkClient {
	// We pass in an external Bitcoin RPC address but otherwise use the default configs.
	btcConfig := btcclient.DefaultBTCConfig()
	// The RPC url must be trimmed to remove the http:// or https:// prefix.
	btcConfig.RPCHost = trimLeadingHttp(opSys.Cfg.DeployConfig.BabylonFinalityGadgetBitcoinRpc)
	sdkClient, err := sdkclient.NewClient(&sdkcfg.Config{
		ChainID:      opSys.Cfg.DeployConfig.BabylonFinalityGadgetChainID,
		ContractAddr: opL2ConsumerConfig.OPFinalityGadgetAddress,
		BTCConfig:    btcConfig,
	})
	require.NoError(t, err)
	return sdkClient
}

func deployCwContract(
	t *testing.T,
	logger *zap.Logger,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	opConsumerId string,
) string {
	cwConfig := opL2ConsumerConfig.ToCosmwasmConfig()
	cwClient, err := opcc.NewCwClient(&cwConfig, logger)
	require.NoError(t, err)

	// store op-finality-gadget contract
	err = cwClient.StoreWasmCode(opFinalityGadgetContractPath)
	require.NoError(t, err)
	opFinalityGadgetContractWasmId, err := cwClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(
		t,
		uint64(1),
		opFinalityGadgetContractWasmId,
		"first deployed contract code_id should be 1",
	)

	// instantiate op contract
	opFinalityGadgetInitMsg := map[string]interface{}{
		"admin":            cwClient.MustGetAddr(),
		"consumer_id":      opConsumerId,
		"activated_height": 0,
		"is_enabled":       true,
	}
	opFinalityGadgetInitMsgBytes, err := json.Marshal(opFinalityGadgetInitMsg)
	require.NoError(t, err)
	err = cwClient.InstantiateContract(opFinalityGadgetContractWasmId, opFinalityGadgetInitMsgBytes)
	require.NoError(t, err)
	listContractsResponse, err := cwClient.ListContractsByCode(
		opFinalityGadgetContractWasmId,
		&sdkquerytypes.PageRequest{},
	)
	require.NoError(t, err)
	require.Len(t, listContractsResponse.Contracts, 1)
	cwContractAddress := listContractsResponse.Contracts[0]
	t.Logf(log.Prefix("op-finality-gadget contract address: %s"), cwContractAddress)
	return cwContractAddress
}

func createLogger(t *testing.T, level zapcore.Level) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	logger, err := config.Build()
	require.NoError(t, err)
	return logger
}

func mockOpL2ConsumerCtrlConfig(nodeDataDir string) *fpcfg.OPStackL2Config {
	dc := bbncfg.DefaultBabylonConfig()

	// fill up the config from dc config
	return &fpcfg.OPStackL2Config{
		Key:            dc.Key,
		ChainID:        dc.ChainID,
		RPCAddr:        dc.RPCAddr,
		GRPCAddr:       dc.GRPCAddr,
		AccountPrefix:  dc.AccountPrefix,
		KeyringBackend: dc.KeyringBackend,
		KeyDirectory:   nodeDataDir,
		GasAdjustment:  1.5,
		GasPrices:      "0.002ubbn",
		Debug:          dc.Debug,
		Timeout:        dc.Timeout,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: dc.OutputFormat,
		SignModeStr:  dc.SignModeStr,
	}
}

func trimLeadingHttp(s string) string {
	t := strings.TrimPrefix(s, "http://")
	return strings.TrimPrefix(t, "https://")
}

func (ctm *OpL2ConsumerTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf(log.Prefix("Babylon node has started"))
}

func (ctm *OpL2ConsumerTestManager) getOpCCAtIndex(i int) *opcc.OPStackL2ConsumerController {
	return ctm.ConsumerFpApps[i].GetConsumerController().(*opcc.OPStackL2ConsumerController)
}

func (ctm *OpL2ConsumerTestManager) WaitForNBlocksAndReturn(
	t *testing.T,
	startHeight uint64,
	n int,
) []*types.BlockInfo {
	var blocks []*types.BlockInfo
	var err error
	require.Eventually(t, func() bool {
		// doesn't matter which FP we use to query blocks. so we use the first consumer FP
		blocks, err = ctm.getOpCCAtIndex(0).QueryBlocks(
			startHeight,
			startHeight+uint64(n-1),
			uint64(n),
		)
		if err != nil || blocks == nil {
			return false
		}
		return len(blocks) == n
	}, e2eutils.EventuallyWaitTimeOut, ctm.getL2BlockTime())
	require.Equal(t, n, len(blocks))
	t.Logf(
		log.Prefix("Successfully waited for %d block(s). The last block's hash at height %d: %s"),
		n,
		blocks[n-1].Height,
		hex.EncodeToString(blocks[n-1].Hash),
	)
	return blocks
}

func (ctm *OpL2ConsumerTestManager) getL1BlockTime() time.Duration {
	return time.Duration(ctm.OpSystem.Cfg.DeployConfig.L1BlockTime) * time.Second
}

func (ctm *OpL2ConsumerTestManager) getL2BlockTime() time.Duration {
	return time.Duration(ctm.OpSystem.Cfg.DeployConfig.L2BlockTime) * time.Second
}

func (ctm *OpL2ConsumerTestManager) WaitForFpVoteAtHeight(
	t *testing.T,
	fpIns *service.FinalityProviderInstance,
	height uint64,
) {
	require.Eventually(t, func() bool {
		lastVotedHeight := fpIns.GetLastVotedHeight()
		return lastVotedHeight >= height
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf(log.Prefix("Fp %s voted at height %d"), fpIns.GetBtcPkHex(), height)
}

/* wait for the target block height that the two FPs both have PubRand commitments
 * the algorithm should be:
 * 1. wait until both FPs have their first PubRand commitments. get the start height of the commitments
 * 2. for the FP that has the smaller start height, wait until it catches up to the other FP's first PubRand commitment
 */
// TODO: there are always two FPs, so we can simplify the logic and data structure. supporting more FPs will require a
// refactor and more complex algorithm
func (ctm *OpL2ConsumerTestManager) WaitForTargetBlockPubRand(
	t *testing.T,
	fpList []*service.FinalityProviderInstance,
) uint64 {
	require.Equal(t, 2, len(fpList), "The below algorithm only supports two FPs")
	var firstFpCommittedPubRand, secondFpCommittedPubRand, targetBlockHeight uint64

	// wait until both FPs have their first PubRand commitments
	require.Eventually(t, func() bool {
		if firstFpCommittedPubRand != 0 && secondFpCommittedPubRand != 0 {
			return true
		}
		if firstFpCommittedPubRand == 0 {
			firstPRCommit, err := queryFirstPublicRandCommit(
				ctm.getOpCCAtIndex(0),
				fpList[0].GetBtcPk(),
			)
			require.NoError(t, err)
			if firstPRCommit != nil {
				firstFpCommittedPubRand = firstPRCommit.StartHeight
			}
		}
		if secondFpCommittedPubRand == 0 {
			secondPRCommit, err := queryFirstPublicRandCommit(
				ctm.getOpCCAtIndex(1),
				fpList[1].GetBtcPk(),
			)
			require.NoError(t, err)
			if secondPRCommit != nil {
				secondFpCommittedPubRand = secondPRCommit.StartHeight
			}
		}
		return false
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	// find the FP's index with the smaller first committed pubrand index in `fpList`
	i := 0
	targetBlockHeight = secondFpCommittedPubRand // the target block is the one with larger start height
	if firstFpCommittedPubRand > secondFpCommittedPubRand {
		i = 1
		targetBlockHeight = firstFpCommittedPubRand
	}

	// wait until the two FPs have overlap in their PubRand commitments
	require.Eventually(t, func() bool {
		committedPubRand, err := ctm.getOpCCAtIndex(i).QueryLastPublicRandCommit(
			fpList[i].GetBtcPk(),
		)
		require.NoError(t, err)
		if committedPubRand == nil {
			return false
		}

		// we found overlap between the two FPs' PubRand commitments
		return committedPubRand.StartHeight+committedPubRand.NumPubRand-1 >= targetBlockHeight
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf(log.Prefix("The target block height is %d"), targetBlockHeight)
	return targetBlockHeight
}

// this works for both Babylon and OP FPs
func (ctm *OpL2ConsumerTestManager) registerSingleFinalityProvider(app *service.FinalityProviderApp, consumerID string, monikerIndex int, t *testing.T) *bbntypes.BIP340PubKey {
	cfg := app.GetConfig()
	keyName := cfg.BabylonConfig.Key
	baseMoniker := fmt.Sprintf("%s-%s", consumerID, e2eutils.MonikerPrefix)
	moniker := fmt.Sprintf("%s%d", baseMoniker, monikerIndex)
	commission := sdkmath.LegacyZeroDec()
	desc := e2eutils.NewDescription(moniker)

	res, err := app.CreateFinalityProvider(
		keyName,
		consumerID,
		e2eutils.Passphrase,
		e2eutils.HdPath,
		desc,
		&commission,
	)
	require.NoError(t, err)
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
	require.NoError(t, err)
	_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
	require.NoError(t, err)
	t.Logf(log.Prefix("Registered Finality Provider %s for %s"), fpPk.MarshalHex(), consumerID)
	return fpPk
}

// - deploy cw contract
// - start op stack system
// - populate the consumer config
// - return the consumer config and the op system
func startExtSystemsAndCreateConsumerCfg(
	t *testing.T,
	logger *zap.Logger,
	bh *e2eutils.BabylonNodeHandler,
) (*fpcfg.OPStackL2Config, *ope2e.System) {
	// create consumer config
	// TODO: using babylon node key dir is a hack. we should fix it
	opL2ConsumerConfig := mockOpL2ConsumerCtrlConfig(bh.GetNodeDataDir())

	// DefaultSystemConfig load the op deploy config from devnet-data folder
	opSysCfg := ope2e.DefaultSystemConfig(t)
	require.Equal(
		t,
		e2eutils.ChainID,
		opSysCfg.DeployConfig.BabylonFinalityGadgetChainID,
		"should be chain-test in devnetL1.json that means to connect with the Babylon localnet",
	)
	opConsumerId := getConsumerChainId(&opSysCfg)

	// deploy op-finality-gadget contract
	cwContractAddress := deployCwContract(t, logger, opL2ConsumerConfig, opConsumerId)

	// replace the contract address
	opSysCfg.DeployConfig.BabylonFinalityGadgetContractAddress = cwContractAddress
	// supress OP system logs
	opSysCfg.Loggers["verifier"] = optestlog.Logger(t, gethlog.LevelError).New("role", "verifier")
	opSysCfg.Loggers["sequencer"] = optestlog.Logger(t, gethlog.LevelError).New("role", "sequencer")
	opSysCfg.Loggers["batcher"] = optestlog.Logger(t, gethlog.LevelError).New("role", "watcher")

	// start op stack system
	opSys, err := opSysCfg.Start(t)
	require.NoError(t, err, "Error starting up op stack system")

	// new op consumer controller
	opL2ConsumerConfig.OPStackL2RPCAddress = opSys.EthInstances["sequencer"].HTTPEndpoint()
	opL2ConsumerConfig.OPFinalityGadgetAddress = cwContractAddress

	return opL2ConsumerConfig, opSys
}

func (ctm *OpL2ConsumerTestManager) waitForConsumerFPRegistration(t *testing.T, n int) {
	require.Eventually(t, func() bool {
		fps, err := ctm.BBNClient.QueryConsumerFinalityProviders(ctm.getConsumerChainId())
		if err != nil {
			t.Logf(log.Prefix("failed to query consumer FP(s) from Babylon %s"), err.Error())
			return false
		}
		if len(fps) != n {
			return false
		}
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
}

type stakingParam struct {
	stakingTime   uint16
	stakingAmount int64
}

// - register a Babylon finality provider
// - register and start n consumer finality providers
// - insert BTC delegations
// - wait until all delegations are active
// - return the list of consumer finality providers
func (ctm *OpL2ConsumerTestManager) SetupFinalityProviders(
	t *testing.T,
	n int, // number of consumer FPs
	stakingParams []stakingParam,
) []*service.FinalityProviderInstance {
	// A BTC delegation has to stake to at least one Babylon finality provider
	// https://github.com/babylonchain/babylon-private/blob/3d8f190c9b0c0795f6546806e3b8582de716cd60/x/btcstaking/keeper/msg_server.go#L220
	// So we have to register a Babylon chain FP
	bbnFpPk := ctm.RegisterBabylonFinalityProvider(t)

	// register consumer chain FPs
	consumerFpPkList := ctm.RegisterConsumerFinalityProvider(t, n)

	// insert BTC delegations
	for i := 0; i < n; i++ {
		ctm.InsertBTCDelegation(
			t,
			[]*btcec.PublicKey{bbnFpPk.MustToBTCPK(), consumerFpPkList[0].MustToBTCPK()},
			stakingParams[i].stakingTime,
			stakingParams[i].stakingAmount,
		)
	}

	// wait until all delegations are active
	ctm.WaitForDelegations(t, n)

	// start consumer chain FPs (has to wait until all delegations are active)
	fpList := ctm.StartConsumerFinalityProvider(t, consumerFpPkList)

	return fpList
}

func (ctm *OpL2ConsumerTestManager) RegisterConsumerFinalityProvider(
	t *testing.T,
	n int,
) []*bbntypes.BIP340PubKey {
	consumerFpPkList := make([]*bbntypes.BIP340PubKey, 0, n)

	for i := 0; i < n; i++ {
		app := ctm.ConsumerFpApps[i]
		fpPk := ctm.registerSingleFinalityProvider(app, ctm.getConsumerChainId(), i, t)
		consumerFpPkList[i] = fpPk
	}

	ctm.waitForConsumerFPRegistration(t, n)
	return consumerFpPkList
}

func (ctm *OpL2ConsumerTestManager) waitForBabylonFPRegistration(t *testing.T) {
	require.Eventually(t, func() bool {
		fps, err := ctm.BBNClient.QueryFinalityProviders()
		if err != nil {
			t.Logf(log.Prefix("failed to query Babylon FP(s) from Babylon %s"), err.Error())
			return false
		}
		// only one Babylon FP should be registered
		if len(fps) != 1 {
			return false
		}
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
}

func (ctm *OpL2ConsumerTestManager) RegisterBabylonFinalityProvider(
	t *testing.T,
) *bbntypes.BIP340PubKey {
	babylonFpPk := ctm.registerSingleFinalityProvider(ctm.BabylonFpApp, e2eutils.ChainID, 0, t)
	ctm.waitForBabylonFPRegistration(t)
	return babylonFpPk
}

func (ctm *OpL2ConsumerTestManager) WaitForNextFinalizedBlock(
	t *testing.T,
	checkedHeight uint64,
) uint64 {
	finalizedBlockHeight := uint64(0)
	require.Eventually(t, func() bool {
		// doesn't matter which FP we use to query. so we use the first consumer FP
		nextFinalizedBlock, err := ctm.getOpCCAtIndex(0).QueryLatestFinalizedBlock()
		require.NoError(t, err)
		finalizedBlockHeight = nextFinalizedBlock.Height
		return finalizedBlockHeight > checkedHeight
	}, e2eutils.EventuallyWaitTimeOut, 5*time.Duration(ctm.OpSystem.Cfg.DeployConfig.L2BlockTime)*time.Second)
	return finalizedBlockHeight
}

func (ctm *OpL2ConsumerTestManager) getConsumerChainId() string {
	return getConsumerChainId(&ctm.OpSystem.Cfg)
}

func getConsumerChainId(opSysCfg *ope2e.SystemConfig) string {
	l2ChainId := opSysCfg.DeployConfig.L2ChainID
	return fmt.Sprintf("%s%d", consumerChainIdPrefix, l2ChainId)
}

func (ctm *OpL2ConsumerTestManager) StartConsumerFinalityProvider(
	t *testing.T,
	fpPkList []*bbntypes.BIP340PubKey,
) []*service.FinalityProviderInstance {
	var resFpList []*service.FinalityProviderInstance

	for i := 0; i < len(fpPkList); i++ {
		app := ctm.ConsumerFpApps[i]
		err := app.StartHandlingFinalityProvider(fpPkList[i], e2eutils.Passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPkList[i])
		resFpList = append(resFpList, fpIns)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)
	}

	return resFpList
}

// query the FP has its first PubRand commitment
func queryFirstPublicRandCommit(
	opcc *opstackl2.OPStackL2ConsumerController,
	fpPk *btcec.PublicKey,
) (*types.PubRandCommit, error) {
	fpPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)
	queryMsg := &opstackl2.QueryMsg{
		FirstPubRandCommit: &opstackl2.PubRandCommit{
			BtcPkHex: fpPubKey.MarshalHex(),
		},
	}

	jsonData, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling to JSON: %w", err)
	}

	stateResp, err := opcc.CwClient.QuerySmartContractState(
		opcc.Cfg.OPFinalityGadgetAddress,
		string(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query smart contract state: %w", err)
	}
	if stateResp.Data == nil {
		return nil, nil
	}

	var resp *types.PubRandCommit
	err = json.Unmarshal(stateResp.Data, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if resp == nil {
		return nil, nil
	}
	if err := resp.Validate(); err != nil {
		return nil, err
	}

	return resp, nil
}

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	t.Log("Stopping test manager")
	var err error
	// FpApp has to stop first or you will get "rpc error: desc = account xxx not found: key not found" error
	// b/c when Babylon daemon is stopped, FP won't be able to find the keyring backend
	err = ctm.BabylonFpApp.Stop()
	require.NoError(t, err)
	t.Log(log.Prefix("Stopped Babylon FP App"))

	for i, app := range ctm.ConsumerFpApps {
		err = app.Stop()
		require.NoError(t, err)
		t.Logf(log.Prefix("Stopped Consumer FP App %d"), i)
	}

	ctm.OpSystem.Close()
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	err = os.RemoveAll(ctm.BaseDir)
	require.NoError(t, err)
}
