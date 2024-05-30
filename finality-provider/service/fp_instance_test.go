package service_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	eotscfg "github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/metrics"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/types"
)

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		currentHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, Hash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(gomock.Any(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()
		_, fpIns, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()

		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(fpIns.GetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		mockClientController.EXPECT().QueryLastCommittedPublicRand(gomock.Any(), uint64(1)).Return(nil, nil).AnyTimes()
		res, err := fpIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
	})
}

func FuzzSubmitFinalitySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		currentHeight := randomStartingHeight + uint64(r.Int63n(10)+1)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, Hash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()
		_, fpIns, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()

		mockClientController.EXPECT().QueryFinalityProviderVotingPower(fpIns.GetBtcPk(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()
		lastCommittedHeight := randomStartingHeight + 25
		lastCommittedPubRandMap := make(map[uint64]*btcec.FieldVal)
		lastCommittedPubRandMap[lastCommittedHeight] = testutil.GenPublicRand(r, t).ToFieldVal()
		mockClientController.EXPECT().QueryLastCommittedPublicRand(gomock.Any(), uint64(1)).Return(lastCommittedPubRandMap, nil).AnyTimes()
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(fpIns.GetBtcPk(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()

		// submit finality sig
		nextBlock := &types.BlockInfo{
			Height: startingBlock.Height + 1,
			Hash:   testutil.GenRandomByteArray(r, 32),
		}
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			SubmitFinalitySig(fpIns.GetBtcPk(), nextBlock.Height, nextBlock.Hash, gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		providerRes, err := fpIns.SubmitFinalitySignature(nextBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, providerRes.TxHash)

		// check the last_voted_height
		require.Equal(t, nextBlock.Height, fpIns.GetLastVotedHeight())
		require.Equal(t, nextBlock.Height, fpIns.GetLastProcessedHeight())
	})
}

func startFinalityProviderAppWithRegisteredFp(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController, startingHeight uint64) (*service.FinalityProviderApp, *service.FinalityProviderInstance, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := eotscfg.DefaultConfigWithHomePath(eotsHomeDir)
	eotsdb, err := eotsCfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg.KeyringBackend, eotsdb, logger)
	require.NoError(t, err)

	// create finality-provider app with randomized config
	fpHomeDir := filepath.Join(t.TempDir(), "fp-home")
	fpCfg := config.DefaultConfigWithHome(fpHomeDir)
	fpCfg.NumPubRand = testutil.TestPubRandNum
	fpCfg.PollerConfig.AutoChainScanningMode = false
	fpCfg.PollerConfig.StaticChainScanningStartHeight = startingHeight
	fpdb, err := fpCfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	app, err := service.NewFinalityProviderApp(&fpCfg, cc, em, fpdb, logger)
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)
	err = app.StartHandlingAll()
	require.NoError(t, err)

	// create registered finality-provider
	fp := testutil.GenStoredFinalityProvider(r, t, app, passphrase, hdPath)
	err = app.GetFinalityProviderStore().SetFpStatus(fp.BtcPk, proto.FinalityProviderStatus_REGISTERED)
	require.NoError(t, err)
	// TODO: use mock metrics
	m := metrics.NewFpMetrics()
	fpIns, err := service.NewFinalityProviderInstance(fp.GetBIP340BTCPK(), &fpCfg, app.GetFinalityProviderStore(), cc, em, m, passphrase, make(chan *service.CriticalError), logger)
	require.NoError(t, err)

	cleanUp := func() {
		err = app.Stop()
		require.NoError(t, err)
		err = eotsdb.Close()
		require.NoError(t, err)
		err = fpdb.Close()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(fpHomeDir)
		require.NoError(t, err)
	}

	return app, fpIns, cleanUp
}
