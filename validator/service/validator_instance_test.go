package service_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/validator/proto"
	"github.com/babylonchain/btc-validator/validator/service"
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
		app, storeValidator, cleanUp := startValidatorAppWithRegisteredValidator(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()
		mockClientController.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()

		valIns, err := app.GetValidatorInstance(storeValidator.MustGetBIP340BTCPK())
		require.NoError(t, err)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(valIns.MustGetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)

		// check the last_committed_height
		numPubRand := app.GetConfig().NumPubRand
		require.Equal(t, startingBlock.Height+numPubRand, valIns.GetStoreValidator().LastCommittedHeight)
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
		app, storeValidator, cleanUp := startValidatorAppWithRegisteredValidator(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()
		mockClientController.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBTCPK(), gomock.Any()).
			Return(uint64(0), nil).AnyTimes()
		valIns, err := app.GetValidatorInstance(storeValidator.MustGetBIP340BTCPK())
		require.NoError(t, err)

		// commit public randomness
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(valIns.MustGetBtcPk(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
		mockClientController.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBTCPK(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()

		// submit finality sig
		nextBlock := &types.BlockInfo{
			Height: startingBlock.Height + 1,
			Hash:   testutil.GenRandomByteArray(r, 32),
		}
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			SubmitFinalitySig(valIns.MustGetBtcPk(), nextBlock.Height, nextBlock.Hash, gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		providerRes, err := valIns.SubmitFinalitySignature(nextBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, providerRes.TxHash)

		// check the last_voted_height
		require.Equal(t, nextBlock.Height, valIns.GetLastVotedHeight())
		require.Equal(t, nextBlock.Height, valIns.GetLastProcessedHeight())
	})
}

func startValidatorAppWithRegisteredValidator(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController, startingHeight uint64) (*service.ValidatorApp, *proto.StoreValidator, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := testutil.GenEOTSConfig(r, t)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg, logger)
	require.NoError(t, err)

	// create validator app with randomized config
	valHomeDir := filepath.Join(t.TempDir(), "val-home")
	valCfg := testutil.GenValConfig(r, t, valHomeDir)
	valCfg.NumPubRand = uint64(25)
	valCfg.ValidatorModeConfig.AutoChainScanningMode = false
	valCfg.ValidatorModeConfig.StaticChainScanningStartHeight = startingHeight
	app, err := service.NewValidatorApp(valHomeDir, valCfg, cc, em, logger)
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	// create registered validator
	validator := testutil.GenStoredValidator(r, t, app, passphrase, hdPath)
	err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
	require.NoError(t, err)
	err = app.StartHandlingValidator(validator.MustGetBIP340BTCPK(), passphrase)
	require.NoError(t, err)

	cleanUp := func() {
		err = app.Stop()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(valHomeDir)
		require.NoError(t, err)
	}

	return app, validator, cleanUp
}
