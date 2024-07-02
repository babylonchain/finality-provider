//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonchain/babylon-da-sdk/sdk"
	bbncfg "github.com/babylonchain/babylon/client/config"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	base_test_manager "github.com/babylonchain/finality-provider/itest/test-manager"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	ope2e "github.com/ethereum-optimism/optimism/op-e2e"
	optestlog "github.com/ethereum-optimism/optimism/op-service/testlog"
	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	opFinalityGadgetContractPath = "../bytecode/op_finality_gadget_48d6604.wasm"
)

type BaseTestManager = base_test_manager.BaseTestManager

type OpL2ConsumerTestManager struct {
	BaseTestManager
	BabylonHandler    *e2eutils.BabylonNodeHandler
	EOTSClient        *client.EOTSManagerGRpcClient
	EOTSConfig        *eotsconfig.Config
	EOTSServerHandler *e2eutils.EOTSServerHandler
	FpApp             *service.FinalityProviderApp
	FpConfig          *fpcfg.Config
	OpL2ConsumerCtrl  *opstackl2.OPStackL2ConsumerController
	BaseDir           string
	SdkClient         *sdk.BabylonQueryClient
	OpSystem          *ope2e.System
	OpChainId         string
}

func StartOpL2ConsumerManager(t *testing.T) *OpL2ConsumerTestManager {
	// Setup consumer test manager
	testDir, err := e2eutils.BaseDir("fpe2etest")
	require.NoError(t, err)

	logger := createLogger(t, zapcore.ErrorLevel)

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2eutils.GenerateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := e2eutils.NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	t.Logf("Fp home dir: %s", fpHomeDir)
	cfg := e2eutils.DefaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	cfg.LogLevel = logger.Level().String()
	cfg.StatusUpdateInterval = 2 * time.Second
	cfg.RandomnessCommitInterval = 2 * time.Second
	cfg.FastSyncInterval = 0 // disable fast sync
	cfg.NumPubRand = 64
	cfg.MinRandHeightGap = 1000
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. start op stack system
	opSysCfg := ope2e.DefaultSystemConfig(t)
	// supress OP system logs
	opSysCfg.Loggers["verifier"] = optestlog.Logger(t, gethlog.LevelError).New("role", "verifier")
	opSysCfg.Loggers["sequencer"] = optestlog.Logger(t, gethlog.LevelError).New("role", "sequencer")
	opSysCfg.Loggers["batcher"] = optestlog.Logger(t, gethlog.LevelError).New("role", "watcher")
	opSys, err := opSysCfg.Start(t)
	require.Nil(t, err, "Error starting up op stack system")

	// 4. register consumer to Babylon
	l2ChainId, err := opSys.Clients["sequencer"].ChainID(context.Background())
	require.NoError(t, err, "failed to get chain ID")
	opConsumerId := fmt.Sprintf("op-stack-l2-%d", l2ChainId.Uint64())
	_, err = bc.RegisterConsumerChain(opConsumerId, "OP consumer chain (test)", "some description about the chain")
	require.NoError(t, err)
	t.Logf("Register consumer %s to Babylon", opConsumerId)

	// 5. new op consumer controller
	opL2Config := mockOpL2ConsumerCtrlConfig(bh.GetNodeDataDir())
	opL2Config.OPStackL2RPCAddress = opSys.EthInstances["sequencer"].HTTPEndpoint()
	cfg.OPStackL2Config = opL2Config
	// TODO: I am a bit worried that this cfg is now used for both BBN and OP FPs
	// which might cause some problems and hard to debug issues
	opcc, err := opstackl2.NewOPStackL2ConsumerController(cfg.OPStackL2Config, logger)
	require.NoError(t, err)

	// 6. store op-finality-gadget contract
	err = storeWasmCode(opcc, opFinalityGadgetContractPath)
	require.NoError(t, err)

	opFinalityGadgetContractWasmId, err := getLatestCodeId(opcc)
	require.NoError(t, err)
	require.Equal(t, uint64(1), opFinalityGadgetContractWasmId, "first deployed contract code_id should be 1")

	// 7. instantiate op contract
	opFinalityGadgetInitMsg := map[string]interface{}{
		"admin":            opcc.CwClient.MustGetAddr(),
		"consumer_id":      opConsumerId,
		"activated_height": 0,
		"is_enabled":       true,
	}
	opFinalityGadgetInitMsgBytes, err := json.Marshal(opFinalityGadgetInitMsg)
	require.NoError(t, err)
	err = instantiateWasmContract(opcc, opFinalityGadgetContractWasmId, opFinalityGadgetInitMsgBytes)
	require.NoError(t, err)
	// get op contract address
	resp, err := opcc.CwClient.ListContractsByCode(opFinalityGadgetContractWasmId, &sdkquerytypes.PageRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Contracts, 1)
	// update the contract address in config to replace a placeholder address
	// previously used to bypass the validation
	opcc.Cfg.OPFinalityGadgetAddress = resp.Contracts[0]
	// only for the e2e test
	sdkCfgChainType := -1

	opSys.RollupConfig.BabylonConfig.ChainType = sdkCfgChainType
	opSys.RollupConfig.BabylonConfig.ContractAddress = resp.Contracts[0]
	t.Logf("Deployed op finality contract address: %s", resp.Contracts[0])

	// 8. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := e2eutils.NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 9. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, opcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	// 10. init SDK client
	sdkClient, err := sdk.NewClient(&sdk.Config{
		ChainType:    sdkCfgChainType,
		ContractAddr: opcc.Cfg.OPFinalityGadgetAddress,
	})
	require.NoError(t, err)

	ctm := &OpL2ConsumerTestManager{
		BaseTestManager:   BaseTestManager{BBNClient: bc, CovenantPrivKeys: covenantPrivKeys},
		BabylonHandler:    bh,
		EOTSClient:        eotsCli,
		EOTSConfig:        eotsCfg,
		EOTSServerHandler: eh,
		FpApp:             fpApp,
		FpConfig:          cfg,
		OpL2ConsumerCtrl:  opcc,
		BaseDir:           testDir,
		SdkClient:         sdkClient,
		OpSystem:          opSys,
		OpChainId:         opConsumerId,
	}

	ctm.WaitForServicesStart(t)
	return ctm
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
		// make random contract address for now to avoid validation errors,
		// later we will update it with the correct address in the test
		OPFinalityGadgetAddress: datagen.GenRandomAccount().GetAddress().String(),
		Key:                     dc.Key,
		ChainID:                 dc.ChainID,
		RPCAddr:                 dc.RPCAddr,
		GRPCAddr:                dc.GRPCAddr,
		AccountPrefix:           dc.AccountPrefix,
		KeyringBackend:          dc.KeyringBackend,
		KeyDirectory:            nodeDataDir,
		GasAdjustment:           1.5,
		GasPrices:               "0.002ubbn",
		Debug:                   dc.Debug,
		Timeout:                 dc.Timeout,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: dc.OutputFormat,
		SignModeStr:  dc.SignModeStr,
	}
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
	t.Logf("Babylon node has started")
}

func (ctm *OpL2ConsumerTestManager) WaitForNBlocksAndReturn(t *testing.T, startHeight uint64, n int) []*types.BlockInfo {
	var (
		blocks []*types.BlockInfo
		err    error
	)
	require.Eventually(t, func() bool {
		blocks, err = ctm.OpL2ConsumerCtrl.QueryBlocks(startHeight, startHeight+uint64(n-1), uint64(n))
		if err != nil || blocks == nil {
			return false
		}
		return len(blocks) == n
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	require.Equal(t, n, len(blocks))
	t.Logf("The last block is %d, %s", blocks[n-1].Height, hex.EncodeToString(blocks[n-1].Hash))
	return blocks
}

func (ctm *OpL2ConsumerTestManager) WaitForFpVoteAtHeight(t *testing.T, fpIns *service.FinalityProviderInstance, height uint64) {
	lastVotedHeight := fpIns.GetLastVotedHeight()
	require.Eventually(t, func() bool {
		if lastVotedHeight >= height {
			return true
		} else {
			lastVotedHeight = fpIns.GetLastVotedHeight()
			return false
		}
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf("Fp %s voted at height %d", fpIns.GetBtcPkHex(), height)
}

func (ctm *OpL2ConsumerTestManager) WaitForTargetBlockPubRand(t *testing.T, fpList []*service.FinalityProviderInstance, requiredBlockOverlapLen uint64) []*uint64 {
	require.Equal(t, 2, len(fpList), "The below algorithm only supports two FPs")
	fpStartHeightList := make([]*uint64, 2)
	require.Eventually(t, func() bool {
		firstFpCommittedPubRand, _ := ctm.OpL2ConsumerCtrl.QueryFirstPublicRandCommit(fpList[0].GetBtcPk())
		secondFpCommittedPubRand, _ := ctm.OpL2ConsumerCtrl.QueryFirstPublicRandCommit(fpList[1].GetBtcPk())

		if fpStartHeightList[0] == nil {
			fpStartHeightList[0] = new(uint64)
			*fpStartHeightList[0] = firstFpCommittedPubRand.StartHeight
		}
		if fpStartHeightList[1] == nil {
			fpStartHeightList[1] = new(uint64)
			*fpStartHeightList[1] = secondFpCommittedPubRand.StartHeight
		}
		// it is possible one FP is falling behind
		if fpStartHeightList[0] == nil || fpStartHeightList[1] == nil {
			return false
		}

		var diff uint64
		if *fpStartHeightList[0] < *fpStartHeightList[1] {
			diff = *fpStartHeightList[1] - *fpStartHeightList[0]
		} else {
			diff = *fpStartHeightList[0] - *fpStartHeightList[1]
		}

		// check the two FP pubrand commitments overlaps
		if diff > ctm.FpConfig.NumPubRand-requiredBlockOverlapLen+1 {
			if *fpStartHeightList[0] < *fpStartHeightList[1] {
				*fpStartHeightList[0] = firstFpCommittedPubRand.StartHeight
			} else {
				*fpStartHeightList[1] = secondFpCommittedPubRand.StartHeight
			}
			return false
		}

		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf("Test block height %d and %d", *fpStartHeightList[0], *fpStartHeightList[1])
	return fpStartHeightList
}

func (ctm *OpL2ConsumerTestManager) RegisterBBNFinalityProvider(t *testing.T) *btcec.PublicKey {
	app := ctm.FpApp
	chainId := e2eutils.ChainID
	fpName := chainId + "-" + e2eutils.FpNamePrefix
	moniker := chainId + "-" + e2eutils.MonikerPrefix
	commission := sdkmath.LegacyZeroDec()
	desc := e2eutils.NewDescription(moniker)
	cfg := app.GetConfig()
	_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, e2eutils.Passphrase, e2eutils.HdPath, "")
	require.NoError(t, err)
	res, err := app.CreateFinalityProvider(fpName, chainId, e2eutils.Passphrase, e2eutils.HdPath, desc, &commission)
	require.NoError(t, err)
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
	require.NoError(t, err)
	_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
	require.NoError(t, err)
	t.Logf("Registered Finality Provider %s for %s", fpPk.MarshalHex(), chainId)
	return fpPk.MustToBTCPK()
}

func (ctm *OpL2ConsumerTestManager) StartFinalityProvider(t *testing.T, isBabylonFp bool, n int) []*service.FinalityProviderInstance {
	app := ctm.FpApp

	chainId := ctm.OpChainId
	if isBabylonFp {
		// While using another mock value, it throws the error: the finality-provider manager has already stopped
		chainId = e2eutils.ChainID
	}

	for i := 0; i < n; i++ {
		fpName := chainId + "-" + e2eutils.FpNamePrefix + strconv.Itoa(i)
		moniker := chainId + "-" + e2eutils.MonikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := e2eutils.NewDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, e2eutils.Passphrase, e2eutils.HdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, chainId, e2eutils.Passphrase, e2eutils.HdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
		t.Logf("Registered Finality Provider %s for %s", fpPk.MarshalHex(), chainId)
		require.NoError(t, err)
		err = app.StartHandlingFinalityProvider(fpPk, e2eutils.Passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPk)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			if isBabylonFp {
				fps, err := ctm.BBNClient.QueryFinalityProviders()
				if err != nil {
					t.Logf("failed to query finality providers from Babylon %s", err.Error())
					return false
				}
				if len(fps) != i+1 {
					return false
				}
				for _, fp := range fps {
					if !strings.Contains(fp.Description.Moniker, e2eutils.MonikerPrefix) {
						return false
					}
				}
			} else {
				fps, err := ctm.BBNClient.QueryConsumerFinalityProviders(ctm.OpChainId)
				if err != nil {
					t.Logf("failed to query finality providers from Babylon %s", err.Error())
					return false
				}
				if len(fps) != i+1 {
					return false
				}
				for _, fp := range fps {
					if !strings.Contains(fp.Description.Moniker, e2eutils.MonikerPrefix) {
						return false
					}
				}
			}
			return true
		}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	}

	fpInsList := app.ListFinalityProviderInstances()
	t.Logf("The test manager is running with %v finality-provider(s)", len(fpInsList))

	var resFpList []*service.FinalityProviderInstance
	for _, fp := range fpInsList {
		if bytes.Equal(fp.GetChainID(), []byte(chainId)) {
			resFpList = append(resFpList, fp)
		}
	}
	require.Equal(t, n, len(resFpList))

	return resFpList
}

func storeWasmCode(opcc *opstackl2.OPStackL2ConsumerController, wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmdtypes.MsgStoreCode{
		Sender:       opcc.CwClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = opcc.ReliablySendMsg(storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func instantiateWasmContract(opcc *opstackl2.OPStackL2ConsumerController, codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: opcc.CwClient.MustGetAddr(),
		Admin:  opcc.CwClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "op-test",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := opcc.ReliablySendMsg(instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// returns the latest wasm code id.
func getLatestCodeId(opcc *opstackl2.OPStackL2ConsumerController) (uint64, error) {
	pagination := &sdkquery.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := opcc.CwClient.ListCodes(pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	t.Log("Stopping test manager")
	var err error
	// FpApp has to stop first or you will get "rpc error: desc = account xxx not found: key not found" error
	// b/c when Babylon daemon is stopped, FP won't be able to find the keyring backend
	err = ctm.FpApp.Stop()
	require.NoError(t, err)
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	ctm.OpSystem.Close()
	err = os.RemoveAll(ctm.BaseDir)
	require.NoError(t, err)
}
