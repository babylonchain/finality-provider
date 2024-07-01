package e2etest_babylon

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	base_test_manager "github.com/babylonchain/finality-provider/itest/test-manager"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/types"
)

type BaseTestManager = base_test_manager.BaseTestManager

type TestManager struct {
	BaseTestManager
	Wg                sync.WaitGroup
	BabylonHandler    *e2eutils.BabylonNodeHandler
	EOTSServerHandler *e2eutils.EOTSServerHandler
	FpConfig          *fpcfg.Config
	EOTSConfig        *eotsconfig.Config
	Fpa               *service.FinalityProviderApp
	EOTSClient        *client.EOTSManagerGRpcClient
	BBNConsumerClient *bbncc.BabylonConsumerController
	baseDir           string
}

func StartManager(t *testing.T) *TestManager {
	testDir, err := e2eutils.BaseDir("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2eutils.GenerateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := e2eutils.NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := e2eutils.DefaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)
	bcc, err := bbncc.NewBabylonConsumerController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := e2eutils.NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 4. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, bcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	tm := &TestManager{
		BaseTestManager:   BaseTestManager{BBNClient: bc, CovenantPrivKeys: covenantPrivKeys},
		BabylonHandler:    bh,
		EOTSServerHandler: eh,
		FpConfig:          cfg,
		EOTSConfig:        eotsCfg,
		Fpa:               fpApp,
		EOTSClient:        eotsCli,
		BBNConsumerClient: bcc,
		baseDir:           testDir,
	}

	tm.WaitForServicesStart(t)

	return tm
}

func (tm *TestManager) WaitForServicesStart(t *testing.T) {
	// wait for Babylon node starts
	require.Eventually(t, func() bool {
		params, err := tm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		tm.StakingParams = params
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf("Babylon node is started")
}

func StartManagerWithFinalityProvider(
	t *testing.T,
	n int,
) (*TestManager, []*service.FinalityProviderInstance) {
	tm := StartManager(t)
	app := tm.Fpa

	for i := 0; i < n; i++ {
		fpName := e2eutils.FpNamePrefix + strconv.Itoa(i)
		moniker := e2eutils.MonikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := e2eutils.NewDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(
			cfg.BabylonConfig.KeyDirectory,
			cfg.BabylonConfig.ChainID,
			fpName,
			keyring.BackendTest,
			e2eutils.Passphrase,
			e2eutils.HdPath,
			"",
		)
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(
			fpName,
			e2eutils.ChainID,
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
		err = app.StartHandlingFinalityProvider(fpPk, e2eutils.Passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPk)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)

		// check finality providers on Babylon side
		require.Eventually(t, func() bool {
			fps, err := tm.BBNClient.QueryFinalityProviders()
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
				if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
					return false
				}
			}

			return true
		}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	}

	fpInsList := app.ListFinalityProviderInstances()
	require.Equal(t, n, len(fpInsList))

	t.Logf("The test manager is running with %v finality-provider(s)", len(fpInsList))

	return tm, fpInsList
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.Fpa.Stop()
	require.NoError(t, err)
	err = tm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.baseDir)
	require.NoError(t, err)
	tm.EOTSServerHandler.Stop()
}

func (tm *TestManager) WaitForFpRegistered(t *testing.T, bbnPk *secp256k1.PubKey) {
	require.Eventually(t, func() bool {
		queriedFps, err := tm.BBNClient.QueryFinalityProviders()
		if err != nil {
			return false
		}
		return len(queriedFps) == 1 && queriedFps[0].BabylonPk.Equals(bbnPk)
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf("the finality-provider is successfully registered")
}

func (tm *TestManager) CheckBlockFinalization(t *testing.T, height uint64, num int) {
	// we need to ensure votes are collected at the given height
	require.Eventually(t, func() bool {
		votes, err := tm.BBNClient.QueryVotesAtHeight(height)
		if err != nil {
			t.Logf("failed to get the votes at height %v: %s", height, err.Error())
			return false
		}
		return len(votes) == num
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	// as the votes have been collected, the block should be finalized
	require.Eventually(t, func() bool {
		finalized, err := tm.BBNConsumerClient.QueryIsBlockFinalized(height)
		if err != nil {
			t.Logf("failed to query block at height %v: %s", height, err.Error())
			return false
		}
		return finalized
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
}

func (tm *TestManager) WaitForFpVoteCast(
	t *testing.T,
	fpIns *service.FinalityProviderInstance,
) uint64 {
	var lastVotedHeight uint64
	require.Eventually(t, func() bool {
		if fpIns.GetLastVotedHeight() > 0 {
			lastVotedHeight = fpIns.GetLastVotedHeight()
			return true
		} else {
			return false
		}
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	return lastVotedHeight
}

func (tm *TestManager) WaitForNFinalizedBlocksAndReturnTipHeight(t *testing.T, n uint) uint64 {
	var (
		firstFinalizedBlock *types.BlockInfo
		err                 error
		lastFinalizedBlock  *types.BlockInfo
	)

	require.Eventually(t, func() bool {
		lastFinalizedBlock, err = tm.BBNConsumerClient.QueryLatestFinalizedBlock()
		if err != nil {
			t.Logf("failed to get the latest finalized block: %s", err.Error())
			return false
		}
		if lastFinalizedBlock == nil {
			return false
		}
		if firstFinalizedBlock == nil {
			firstFinalizedBlock = lastFinalizedBlock
		}
		return lastFinalizedBlock.Height-firstFinalizedBlock.Height >= uint64(n-1)
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf("the block is finalized at %v", lastFinalizedBlock.Height)

	return lastFinalizedBlock.Height
}

func (tm *TestManager) WaitForFpShutDown(t *testing.T, pk *bbntypes.BIP340PubKey) {
	require.Eventually(t, func() bool {
		_, err := tm.Fpa.GetFinalityProviderInstance(pk)
		return err != nil
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Logf("the finality-provider instance %s is shutdown", pk.MarshalHex())
}

func (tm *TestManager) StopAndRestartFpAfterNBlocks(
	t *testing.T,
	n uint,
	fpIns *service.FinalityProviderInstance,
) {
	blockBeforeStopHeight, err := tm.BBNConsumerClient.QueryLatestBlockHeight()
	require.NoError(t, err)
	err = fpIns.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		headerAfterStopHeight, err := tm.BBNConsumerClient.QueryLatestBlockHeight()
		if err != nil {
			return false
		}

		return headerAfterStopHeight >= uint64(n)+blockBeforeStopHeight
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	t.Log("restarting the finality-provider instance")

	tm.FpConfig.PollerConfig.AutoChainScanningMode = true
	err = fpIns.Start()
	require.NoError(t, err)
}

func (tm *TestManager) GetFpPrivKey(t *testing.T, fpPk []byte) *btcec.PrivateKey {
	record, err := tm.EOTSClient.KeyRecord(fpPk, e2eutils.Passphrase)
	require.NoError(t, err)
	return record.PrivKey
}

func (tm *TestManager) InsertWBTCHeaders(t *testing.T, r *rand.Rand) {
	params, err := tm.BBNClient.QueryStakingParams()
	require.NoError(t, err)
	btcTipResp, err := tm.BBNClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(btcTipResp.HeaderHex)
	require.NoError(t, err)
	kHeaders := datagen.NewBTCHeaderChainFromParentInfo(r, &btclctypes.BTCHeaderInfo{
		Header: &tipHeader,
		Hash:   tipHeader.Hash(),
		Height: btcTipResp.Height,
		Work:   &btcTipResp.Work,
	}, uint32(params.FinalizationTimeoutBlocks))
	_, err = tm.BBNClient.InsertBtcBlockHeaders(kHeaders.ChainToBytes())
	require.NoError(t, err)
}
