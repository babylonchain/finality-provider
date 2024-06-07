package e2etest

import (
	"strconv"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	wasmdapp "github.com/CosmWasm/wasmd/app"
	bbntypes "github.com/babylonchain/babylon/types"
	fpcc "github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type ConsumerTestManager struct {
	*TestManager
	WasmdHandler        *WasmdNodeHandler
	WasmdConsumerClient *fpcc.WasmdConsumerController
}

func StartConsumerManager(t *testing.T) *ConsumerTestManager {
	// Setup test manager
	tm := StartManager(t)

	// Start wasmd node
	wh := NewWasmdNodeHandler(t)
	err := wh.Start()
	require.NoError(t, err)

	// Setup wasmd consumer client
	logger := zap.NewNop()
	tm.FpConfig.WasmdConfig = config.DefaultWasmdConfig()
	tm.FpConfig.WasmdConfig.KeyDirectory = wh.dataDir
	encodingConfig := wasmdapp.MakeEncodingConfig(t)
	wcc, err := fpcc.NewWasmdConsumerController(tm.FpConfig.WasmdConfig, encodingConfig, logger)
	require.NoError(t, err)

	ctm := &ConsumerTestManager{
		TestManager:         tm,
		WasmdHandler:        wh,
		WasmdConsumerClient: wcc,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func (ctm *ConsumerTestManager) WaitForServicesStart(t *testing.T) {
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
	ctm.TestManager.Stop(t)
	ctm.WasmdHandler.Stop(t)
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
