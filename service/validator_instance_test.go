package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := testutil.PrepareMockedClientController(t, startingBlock.Height, startingBlock.LastCommitHash)
		app, storeValidator, cleanUp := newValidatorAppWithRegisteredValidator(t, r, mockBabylonClient)
		defer cleanUp()
		mockBabylonClient.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBIP340BTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()
		err := app.Start()
		require.NoError(t, err)

		valIns, err := app.GetValidatorInstance(storeValidator.GetBabylonPK())
		require.NoError(t, err)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(valIns.GetBtcPkBIP340(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(valIns.GetBtcPkBIP340()).
			Return(uint64(0), nil).AnyTimes()
		res, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)

		// check the last_committed_height
		numPubRand := app.GetConfig().NumPubRand
		require.Equal(t, startingBlock.Height+numPubRand, valIns.GetStoreValidator().LastCommittedHeight)

		// check the committed pub rand
		randPairs, err := valIns.GetCommittedPubRandPairList()
		require.NoError(t, err)
		require.Equal(t, int(numPubRand), len(randPairs))
	})
}

func FuzzSubmitFinalitySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := testutil.PrepareMockedClientController(t, startingBlock.Height, startingBlock.LastCommitHash)
		app, storeValidator, cleanUp := newValidatorAppWithRegisteredValidator(t, r, mockBabylonClient)
		defer cleanUp()
		mockBabylonClient.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBIP340BTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()
		err := app.Start()
		require.NoError(t, err)
		valIns, err := app.GetValidatorInstance(storeValidator.GetBabylonPK())
		require.NoError(t, err)

		// commit public randomness
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(valIns.GetBtcPkBIP340(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(valIns.GetBtcPkBIP340()).
			Return(uint64(0), nil).AnyTimes()
		res, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
		mockBabylonClient.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBIP340BTCPK(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()

		// submit finality sig
		nextBlock := &types.BlockInfo{
			Height:         startingBlock.Height + 1,
			LastCommitHash: testutil.GenRandomByteArray(r, 32),
		}
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		mockBabylonClient.EXPECT().
			SubmitFinalitySig(valIns.GetBtcPkBIP340(), nextBlock.Height, nextBlock.LastCommitHash, gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err = valIns.SubmitFinalitySignature(nextBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)

		// check the last_voted_height
		require.Equal(t, nextBlock.Height, valIns.GetLastVotedHeight())
	})
}

func newValidatorAppWithRegisteredValidator(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController) (*service.ValidatorApp, *proto.StoreValidator, func()) {
	// create validator app with config
	cfg := valcfg.DefaultConfig()
	cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
	cfg.BabylonConfig.KeyDirectory = t.TempDir()
	cfg.NumPubRand = uint64(r.Intn(10) + 1)
	logger := logrus.New()
	app, err := service.NewValidatorAppFromConfig(&cfg, logger, cc)
	require.NoError(t, err)

	// create registered validator
	validator := testutil.GenStoredValidator(r, t, app)
	err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
	require.NoError(t, err)
	config := app.GetConfig()

	cleanUp := func() {
		err = app.Stop()
		require.NoError(t, err)
		err := os.RemoveAll(config.DatabaseConfig.Path)
		require.NoError(t, err)
		err = os.RemoveAll(config.BabylonConfig.KeyDirectory)
		require.NoError(t, err)
	}

	return app, validator, cleanUp
}
