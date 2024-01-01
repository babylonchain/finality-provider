package service_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/service"
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
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight, &types.StakingParams{})
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()
		app, storeFp, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(storeFp.MustGetBTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()

		fpIns, err := app.GetFinalityProviderInstance(storeFp.MustGetBIP340BTCPK())
		require.NoError(t, err)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(fpIns.MustGetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
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
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight, &types.StakingParams{})
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()
		app, storeFp, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(storeFp.MustGetBTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()
		fpIns, err := app.GetFinalityProviderInstance(storeFp.MustGetBIP340BTCPK())
		require.NoError(t, err)

		// commit public randomness
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(fpIns.MustGetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err := fpIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(storeFp.MustGetBTCPK(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()

		// submit finality sig
		nextBlock := &types.BlockInfo{
			Height: startingBlock.Height + 1,
			Hash:   testutil.GenRandomByteArray(r, 32),
		}
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			SubmitFinalitySig(fpIns.MustGetBtcPk(), nextBlock.Height, nextBlock.Hash, gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		providerRes, err := fpIns.SubmitFinalitySignature(nextBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, providerRes.TxHash)

		// check the last_voted_height
		require.Equal(t, nextBlock.Height, fpIns.GetLastVotedHeight())
		require.Equal(t, nextBlock.Height, fpIns.GetLastProcessedHeight())
	})
}

func startFinalityProviderAppWithRegisteredFp(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController, startingHeight uint64) (*service.FinalityProviderApp, *proto.StoreFinalityProvider, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := testutil.GenEOTSConfig(r, t)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg, logger)
	require.NoError(t, err)

	// create finality-provider app with randomized config
	fpHomeDir := filepath.Join(t.TempDir(), "fp-home")
	fpCfg := testutil.GenFpConfig(r, t, fpHomeDir)
	fpCfg.NumPubRand = testutil.TestPubRandNum
	fpCfg.PollerConfig.AutoChainScanningMode = false
	fpCfg.PollerConfig.StaticChainScanningStartHeight = startingHeight
	app, err := service.NewFinalityProviderApp(fpHomeDir, fpCfg, cc, em, logger)
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	// create registered finality-provider
	fp := testutil.GenStoredFinalityProvider(r, t, app, passphrase, hdPath)
	err = app.GetFinalityProviderStore().SetFinalityProviderStatus(fp, proto.FinalityProviderStatus_REGISTERED)
	require.NoError(t, err)
	err = app.StartHandlingFinalityProvider(fp.MustGetBIP340BTCPK(), passphrase)
	require.NoError(t, err)

	cleanUp := func() {
		err = app.Stop()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(fpHomeDir)
		require.NoError(t, err)
	}

	return app, fp, cleanUp
}
