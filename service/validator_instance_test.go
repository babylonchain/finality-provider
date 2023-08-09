package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/valcfg"
)

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		startingBlock := &service.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := testutil.PrepareMockedBabylonClient(t, startingBlock.Height, startingBlock.LastCommitHash)
		app, valIns, cleanUp := newValidatorAppWithRegisteredValidator(t, r, mockBabylonClient)
		defer cleanUp()
		err := app.Start()
		require.NoError(t, err)

		expectedTxHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(valIns.GetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(expectedTxHash, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(valIns.GetBtcPk()).
			Return(uint64(0), nil).AnyTimes()
		actualTxHash, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, actualTxHash)

		// check the last_committed_height
		numPubRand := app.GetConfig().NumPubRand
		require.Equal(t, startingBlock.Height+numPubRand, valIns.GetValidatorStored().LastCommittedHeight)

		// check the committed pub rand
		randPairs, err := valIns.GetCommittedPubRandPairList()
		require.NoError(t, err)
		require.Equal(t, int(numPubRand), len(randPairs))
	})
}

// func FuzzSubmitFinalitySig(f *testing.F) {
// 	testutil.AddRandomSeedsToFuzzer(f, 10)
// 	f.Fuzz(func(t *testing.T, seed int64) {
// 		r := rand.New(rand.NewSource(seed))
//
// 		// create validator app with db and mocked Babylon client
// 		cfg := valcfg.DefaultConfig()
// 		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
// 		cfg.BabylonConfig.KeyDirectory = t.TempDir()
// 		defer func() {
// 			err := os.RemoveAll(cfg.DatabaseConfig.Path)
// 			require.NoError(t, err)
// 			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
// 			require.NoError(t, err)
// 		}()
// 		startingBlock := &service.BlockInfo{Height: 1, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
// 		mockBabylonClient := prepareMockedBabylonClient(t, startingBlock)
// 		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
// 		require.NoError(t, err)
//
// 		err = app.Start()
// 		require.NoError(t, err)
// 		defer func() {
// 			err = app.Stop()
// 			require.NoError(t, err)
// 		}()
//
// 		// create a validator object and save it to db
// 		validator := createValidator(r, t, app)
// 		err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
// 		require.NoError(t, err)
// 		btcPkBIP340 := validator.MustGetBIP340BTCPK()
//
// 		// commit public randomness
// 		txHash := testutil.GenRandomByteArray(r, 32)
// 		mockBabylonClient.EXPECT().
// 			CommitPubRandList(btcPkBIP340, startingBlock.Height+1, gomock.Any(), gomock.Any()).
// 			Return(txHash, nil).AnyTimes()
// 		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(btcPkBIP340).
// 			Return(uint64(0), nil).AnyTimes()
// 		txHashes, err := app.CommitPubRandForAll(startingBlock)
// 		require.NoError(t, err)
// 		require.Equal(t, txHash, txHashes[0])
//
// 		// check the committed pub rand
// 		randPairs, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
// 		require.NoError(t, err)
// 		require.Equal(t, int(cfg.NumPubRand), len(randPairs))
//
// 		// submit finality sig
// 		nextBlock := &service.BlockInfo{
// 			Height:         startingBlock.Height + 1,
// 			LastCommitHash: testutil.GenRandomByteArray(r, 32),
// 		}
// 		txHash = testutil.GenRandomByteArray(r, 32)
// 		mockBabylonClient.EXPECT().
// 			SubmitFinalitySig(btcPkBIP340, nextBlock.Height, nextBlock.LastCommitHash, gomock.Any()).
// 			Return(txHash, nil, nil).AnyTimes()
// 		mockBabylonClient.EXPECT().QueryValidatorVotingPower(btcPkBIP340, nextBlock.Height).
// 			Return(uint64(1), nil).AnyTimes()
// 		txHashes, err = app.SubmitFinalitySignaturesForAll(nextBlock)
// 		require.NoError(t, err)
// 		require.Equal(t, txHash, txHashes[0])
//
// 		// check the last_voted_height
// 		updatedVal, err := app.GetValidatorInstance(validator.BabylonPk)
// 		require.NoError(t, err)
// 		require.Equal(t, nextBlock.Height, updatedVal.LastVotedHeight)
// 	})
// }

func newValidatorAppWithRegisteredValidator(t *testing.T, r *rand.Rand, bc babylonclient.BabylonClient) (*service.ValidatorApp, *service.ValidatorInstance, func()) {
	// create validator app with config
	cfg := valcfg.DefaultConfig()
	cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
	cfg.BabylonConfig.KeyDirectory = t.TempDir()
	cfg.NumPubRand = uint64(r.Intn(10) + 1)
	logger := logrus.New()
	app, err := service.NewValidatorAppFromConfig(&cfg, logger, bc)
	require.NoError(t, err)

	// create registered validator
	validator := testutil.GenStoredValidator(r, t, app)
	err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
	require.NoError(t, err)
	config := app.GetConfig()
	valIns, err := service.NewValidatorInstance(validator.GetBabylonPK(), config, app.GetValidatorStore(), app.GetKeyring(), bc, logger)
	require.NoError(t, err)
	err = app.AddValidatorInstance(valIns)
	require.NoError(t, err)

	cleanUp := func() {
		err = app.Stop()
		require.NoError(t, err)
		err := os.RemoveAll(config.DatabaseConfig.Path)
		require.NoError(t, err)
		err = os.RemoveAll(config.BabylonConfig.KeyDirectory)
		require.NoError(t, err)
	}

	return app, valIns, cleanUp
}
