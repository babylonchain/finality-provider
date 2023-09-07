package service_test

import (
	"math/rand"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
)

func FuzzCatchUp(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		currentHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, currentHeight, startingBlock.LastCommitHash)
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(nil, nil)
		app, storeValidator, cleanUp := newValidatorAppWithRegisteredValidator(t, r, mockClientController)
		defer cleanUp()
		err := app.Start()
		require.NoError(t, err)
		valIns, err := app.GetValidatorInstance(storeValidator.GetBabylonPK())
		require.NoError(t, err)

		// commit public randomness
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			CommitPubRandList(valIns.GetBtcPkBIP340(), startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		mockClientController.EXPECT().QueryHeightWithLastPubRand(valIns.GetBtcPkBIP340()).
			Return(uint64(0), nil).AnyTimes()
		res, err := valIns.CommitPubRand(startingBlock)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
		mockClientController.EXPECT().QueryValidatorVotingPower(storeValidator.MustGetBIP340BTCPK(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()
		err = valIns.SetLastCommittedHeight(currentHeight + 1)
		require.NoError(t, err)

		// catch up
		currentBlock := &types.BlockInfo{Height: currentHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		catchUpBlocks := testutil.GenBlocks(r, randomStartingHeight+1, currentHeight)
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		finalizedBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return([]*types.BlockInfo{finalizedBlock}, nil).AnyTimes()
		mockClientController.EXPECT().QueryBlocks(randomStartingHeight+1, currentHeight).
			Return(catchUpBlocks, nil)
		mockClientController.EXPECT().SubmitBatchFinalitySigs(valIns.GetBtcPkBIP340(), catchUpBlocks, gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err = valIns.TryCatchUp(currentBlock)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, expectedTxHash, res.TxHash)
		require.Equal(t, currentHeight, valIns.GetLastVotedHeight())
	})
}
