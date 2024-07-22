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
	FpApp             []*service.FinalityProviderApp
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

	// init SDK client
	sdkClient := initSdkClient(opSys, opL2ConsumerConfig, t)

	// start multiple FPs. each FP has its own EOTS manager and finality provider app
	// there is one Babylon FP and multiple Consumer FPs
	fpApps, eotsHandler := createMultiFps(
		testDir,
		bh,
		opL2ConsumerConfig,
		numOfConsumerFPs+1,
		logger,
		t,
	)

	// register consumer to Babylon (only one of the FPs needs to do it)
	opConsumerId := getConsumerChainId(&opSys.Cfg)
	babylonClient := fpApps[0].GetBabylonController().(*bbncc.BabylonController)
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
		FpApp:             fpApps,
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
	numOfFPs uint8,
	logger *zap.Logger,
	t *testing.T,
) ([]*service.FinalityProviderApp, *e2eutils.EOTSServerHandler) {
	// prepare FPs configs
	fpConfigs := createFpConfigs(testDir, bh, opL2ConsumerConfig, numOfFPs, logger, t)

	// prepare EOTS managers
	eotsHandler, eotsClients := startEotsManagers(testDir, t, fpConfigs, numOfFPs, logger)

	// prepare fpApps
	fpApps := createFpApps(numOfFPs, fpConfigs, eotsClients, logger, t)

	return fpApps, eotsHandler
}

func createFpApps(
	numOfFPs uint8,
	fpConfigs []*fpcfg.Config,
	eotsClients []*client.EOTSManagerGRpcClient,
	logger *zap.Logger,
	t *testing.T,
) []*service.FinalityProviderApp {
	fpApps := make([]*service.FinalityProviderApp, 0, numOfFPs)

	for i := 0; i < int(numOfFPs); i++ {
		cfg := fpConfigs[i]
		eotsCli := eotsClients[i]

		var cc api.ConsumerController
		var err error
		if i == 0 {
			cc, err = bbncc.NewBabylonConsumerController(
				cfg.BabylonConfig,
				&cfg.BTCNetParams,
				logger,
			)
		} else {
			cc, err = opstackl2.NewOPStackL2ConsumerController(cfg.OPStackL2Config, logger)
		}
		require.NoError(t, err)

		bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
		require.NoError(t, err)

		fpdb, err := cfg.DatabaseConfig.GetDbBackend()
		require.NoError(t, err)
		fpApp, err := service.NewFinalityProviderApp(cfg, bc, cc, eotsCli, fpdb, logger)
		require.NoError(t, err)
		err = fpApp.Start()
		require.NoError(t, err)
		t.Logf(log.Prefix("Started FP App %d"), i)

		fpApps = append(fpApps, fpApp)
	}
	return fpApps
}

// create configs for multiple FPs. the first config is for a Babylon FP
func createFpConfigs(
	testDir string,
	bh *e2eutils.BabylonNodeHandler,
	opL2ConsumerConfig *fpcfg.OPStackL2Config,
	numOfFPs uint8,
	logger *zap.Logger,
	t *testing.T,
) []*fpcfg.Config {
	fpConfigs := make([]*fpcfg.Config, 0, numOfFPs)

	for i := 0; i < int(numOfFPs); i++ {
		fpHomeDir := filepath.Join(testDir, fmt.Sprintf("fp-home-%d", i))
		t.Logf(log.Prefix("Fp home dir: %s"), fpHomeDir)

		// customize ports
		// FP default RPC port is 12581, EOTS default RPC port i is 12582
		// FP default metrics port is 2112, EOTS default metrics port is 2113
		cfg := e2eutils.DefaultFpConfigWithPorts(
			"this should be the keyring dir", // this will be replaced below
			fpHomeDir,
			fpcfg.DefaultRPCPort-i,
			metrics.DefaultFpConfig().Port-i,
			eotsconfig.DefaultRPCPort+i,
		)
		cfg.LogLevel = logger.Level().String()
		cfg.StatusUpdateInterval = 2 * time.Second
		cfg.RandomnessCommitInterval = 2 * time.Second
		cfg.NumPubRand = 64
		cfg.MinRandHeightGap = 1000

		// customize key
		cfg.BabylonConfig.KeyDirectory = filepath.Join(testDir, fmt.Sprintf("fp-home-keydir%d", i))
		fpBbnKeyInfo, err := service.CreateChainKey(
			cfg.BabylonConfig.KeyDirectory,
			cfg.BabylonConfig.ChainID,
			cfg.BabylonConfig.Key,
			cfg.BabylonConfig.KeyringBackend,
			e2eutils.Passphrase,
			e2eutils.HdPath,
			"",
		)
		require.NoError(t, err)

		// add some funds for new fp pay for fees '-'
		fundBBNAddr(bh, fpBbnKeyInfo, t)

		if i != 0 { // the first FP is Babylon FP, skip
			opcc := *opL2ConsumerConfig
			opcc.KeyDirectory = cfg.BabylonConfig.KeyDirectory
			opcc.Key = cfg.BabylonConfig.Key
			cfg.OPStackL2Config = &opcc
		}

		fpConfigs = append(fpConfigs, cfg)
	}

	return fpConfigs
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
	cfgs []*fpcfg.Config,
	numOfFPs uint8,
	logger *zap.Logger,
) (*e2eutils.EOTSServerHandler, []*client.EOTSManagerGRpcClient) {
	eotsClients := make([]*client.EOTSManagerGRpcClient, 0, numOfFPs)
	eotsHomeDirs := make([]string, 0, numOfFPs)
	eotsConfigs := make([]*eotsconfig.Config, 0, numOfFPs)

	// start EOTS servers
	for i := 0; i < int(numOfFPs); i++ {
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
	for i := 0; i < int(numOfFPs); i++ {
		// wait for EOTS servers to start
		// see https://github.com/babylonchain/finality-provider/pull/517
		require.Eventually(t, func() bool {
			eotsCli, err := client.NewEOTSManagerGRpcClient(cfgs[i].EOTSManagerAddress)
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
	return ctm.FpApp[i].GetConsumerController().(*opcc.OPStackL2ConsumerController)
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
		blocks, err = ctm.getOpCCAtIndex(1).QueryBlocks(
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
				ctm.getOpCCAtIndex(1),
				fpList[0].GetBtcPk(),
			)
			require.NoError(t, err)
			if firstPRCommit != nil {
				firstFpCommittedPubRand = firstPRCommit.StartHeight
			}
		}
		if secondFpCommittedPubRand == 0 {
			secondPRCommit, err := queryFirstPublicRandCommit(
				ctm.getOpCCAtIndex(2),
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
	i := 1
	targetBlockHeight = secondFpCommittedPubRand // the target block is the one with larger start height
	if firstFpCommittedPubRand > secondFpCommittedPubRand {
		i = 2
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

// can be used for both the Babylon and Consumer FPs
func (ctm *OpL2ConsumerTestManager) registerFinalityProvider(
	t *testing.T,
	consumerID string,
	n int,
	offset int,
) []*bbntypes.BIP340PubKey {
	fpPkList := make([]*bbntypes.BIP340PubKey, 0, n)

	for i := offset; i < n+offset; i++ {
		app := ctm.FpApp[i]
		cfg := app.GetConfig()
		keyName := cfg.BabylonConfig.Key
		baseMoniker := fmt.Sprintf("%s-%s", consumerID, e2eutils.MonikerPrefix)
		moniker := fmt.Sprintf("%s%d", baseMoniker, i)
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
		fpPkList = append(fpPkList, fpPk)
		t.Logf(log.Prefix("Registered Finality Provider %s for %s"), fpPk.MarshalHex(), consumerID)
	}

	return fpPkList
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
// - register and start consumer finality providers
// - insert BTC delegations
// - wait until all delegations are active
// - return the list of finality providers
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
			[]*btcec.PublicKey{bbnFpPk[0].MustToBTCPK(), consumerFpPkList[0].MustToBTCPK()},
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
	consumerFpPkList := ctm.registerFinalityProvider(t, ctm.getConsumerChainId(), n, 1)
	ctm.waitForConsumerFPRegistration(t, n)
	return consumerFpPkList
}

func (ctm *OpL2ConsumerTestManager) waitForBabylonFPRegistration(t *testing.T, n int) {
	require.Eventually(t, func() bool {
		fps, err := ctm.BBNClient.QueryFinalityProviders()
		if err != nil {
			t.Logf(log.Prefix("failed to query Babylon FP(s) from Babylon %s"), err.Error())
			return false
		}
		if len(fps) != n {
			return false
		}
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
}

func (ctm *OpL2ConsumerTestManager) RegisterBabylonFinalityProvider(
	t *testing.T,
) []*bbntypes.BIP340PubKey {
	babylonFpPkList := ctm.registerFinalityProvider(t, e2eutils.ChainID, 1, 0)
	ctm.waitForBabylonFPRegistration(t, 1)
	return babylonFpPkList
}

func (ctm *OpL2ConsumerTestManager) WaitForNextFinalizedBlock(
	t *testing.T,
	checkedHeight uint64,
) uint64 {
	finalizedBlockHeight := uint64(0)
	require.Eventually(t, func() bool {
		// doesn't matter which FP we use to query. so we use the first consumer FP
		nextFinalizedBlock, err := ctm.getOpCCAtIndex(1).QueryLatestFinalizedBlock()
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
		// consumer chain FPs stars from index 1. so i+1 is the index of the FP app
		app := ctm.FpApp[i+1]
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
	for i := 0; i < len(ctm.FpApp); i++ {
		err = ctm.FpApp[i].Stop()
		require.NoError(t, err)
		t.Logf(log.Prefix("Stopped FP App %d"), i)
	}
	require.NoError(t, err)
	ctm.OpSystem.Close()
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	err = os.RemoveAll(ctm.BaseDir)
	require.NoError(t, err)
}
