package e2etest

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	sdklogs "cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	bbntypes "github.com/babylonchain/babylon/types"
	fpcc "github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type ConsumerTestManager struct {
	BabylonHandler      *BabylonNodeHandler
	FpConfig            *fpcfg.Config
	BBNClient           *fpcc.BabylonController
	WasmdHandler        *WasmdNodeHandler
	WasmdConsumerClient *fpcc.CosmwasmConsumerController
	StakingParams       *types.StakingParams
	EOTSServerHandler   *EOTSServerHandler
	EOTSConfig          *eotsconfig.Config
	Fpa                 *service.FinalityProviderApp
	EOTSClient          *client.EOTSManagerGRpcClient
	CovenantPrivKeys    []*btcec.PrivateKey
	baseDir             string
}

func StartConsumerManager(t *testing.T) *ConsumerTestManager {
	// Setup test manager
	testDir, err := tempDirWithName("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := generateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := defaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := fpcc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. setup wasmd node
	wh := NewWasmdNodeHandler(t)
	err = wh.Start()
	require.NoError(t, err)
	cfg.CosmwasmConfig = config.DefaultCosmwasmConfig()
	cfg.CosmwasmConfig.KeyDirectory = wh.dataDir
	cfg.ChainName = fpcc.WasmdConsumerChainName
	tempApp := wasmapp.NewWasmApp(sdklogs.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(t.TempDir()), []wasmkeeper.Option{})
	encodingConfig := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	wcc, err := fpcc.NewCosmwasmConsumerController(cfg.CosmwasmConfig, encodingConfig, logger)
	require.NoError(t, err)

	// 4. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 5. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, wcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	// TODO: setup fp app after contract supports relevant queries

	ctm := &ConsumerTestManager{
		BabylonHandler:      bh,
		FpConfig:            cfg,
		BBNClient:           bc,
		WasmdHandler:        wh,
		WasmdConsumerClient: wcc,
		EOTSServerHandler:   eh,
		EOTSConfig:          eotsCfg,
		Fpa:                 fpApp,
		EOTSClient:          eotsCli,
		CovenantPrivKeys:    covenantPrivKeys,
		baseDir:             testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func (ctm *ConsumerTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Babylon node is started")

	// wait for wasmd to start
	require.Eventually(t, func() bool {
		blockHeight, err := ctm.WasmdHandler.GetLatestBlockHeight()
		if err != nil {
			t.Logf("failed to get latest block height from wasmd %s", err.Error())
			return false
		}
		return blockHeight > 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Wasmd node is started")
}

func (ctm *ConsumerTestManager) Stop(t *testing.T) {
	err := ctm.Fpa.Stop()
	require.Error(t, err) // TODO: error is expected here, as the fp manager is not started and we are trying to stop it
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	ctm.WasmdHandler.Stop(t)
	err = os.RemoveAll(ctm.baseDir)
	require.NoError(t, err)
}

func StartConsumerManagerWithFps(t *testing.T, n int) (*ConsumerTestManager, []*service.FinalityProviderInstance) {
	ctm := StartConsumerManager(t)
	app := ctm.Fpa

	for i := 0; i < n; i++ {
		fpName := fpNamePrefix + strconv.Itoa(i)
		moniker := monikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := newDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, passphrase, hdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, chainID, passphrase, hdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
		require.NoError(t, err)
		err = app.StartHandlingFinalityProvider(fpPk, passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPk)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)

		// check finality providers on Babylon side
		require.Eventually(t, func() bool {
			fps, err := ctm.BBNClient.QueryFinalityProviders()
			if err != nil {
				t.Logf("failed to query finality providers from Babylon %s", err.Error())
				return false
			}

			if len(fps) != i+1 {
				return false
			}

			for _, fp := range fps {
				if !strings.Contains(fp.Description.Moniker, monikerPrefix) {
					return false
				}
				if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
					return false
				}
			}

			return true
		}, eventuallyWaitTimeOut, eventuallyPollTime)
	}

	fpInsList := app.ListFinalityProviderInstances()
	require.Equal(t, n, len(fpInsList))

	t.Logf("the consumer test manager is running with %v finality-provider(s)", len(fpInsList))

	return ctm, fpInsList
}

func (ctm *ConsumerTestManager) CreateFinalityProvidersForChain(t *testing.T, chainID string, n int) []*service.FinalityProviderInstance {
	app := ctm.Fpa
	cfg := app.GetConfig()

	// register all finality providers
	fpPKs := make([]*bbntypes.BIP340PubKey, 0, n)
	for i := 0; i < n; i++ {
		fpName := fpNamePrefix + chainID + "-" + strconv.Itoa(i)
		moniker := monikerPrefix + chainID + "-" + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := newDescription(moniker)
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, chainID, fpName, keyring.BackendTest, passphrase, hdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, chainID, passphrase, hdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		fpPKs = append(fpPKs, fpPk)
		_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
		require.NoError(t, err)
	}

	for i := 0; i < n; i++ {
		// start
		err := app.StartHandlingFinalityProvider(fpPKs[i], passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPKs[i])
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)
	}

	// check finality providers on Babylon side
	require.Eventually(t, func() bool {
		fps, err := ctm.BBNClient.QueryFinalityProviders()
		if err != nil {
			t.Logf("failed to query finality providers from Babylon %s", err.Error())
			return false
		}

		if len(fps) != n {
			return false
		}

		for _, fp := range fps {
			if !strings.Contains(fp.Description.Moniker, monikerPrefix) {
				return false
			}
			if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
				return false
			}
		}

		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	fpInsList := app.ListFinalityProviderInstancesForChain(chainID)
	require.Equal(t, n, len(fpInsList))

	t.Logf("the consumer test manager is running with %v finality-provider(s)", len(fpInsList))

	return fpInsList
}
