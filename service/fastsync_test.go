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

func FuzzFastSync(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		finalizedHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		currentHeight := finalizedHeight + uint64(r.Int63n(10)+1)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil)
		app, storeValidator, cleanUp := newValidatorAppWithRegisteredValidator(t, r, mockClientController, randomStartingHeight)
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

		// fast sync
		catchUpBlocks := testutil.GenBlocks(r, finalizedHeight+1, currentHeight)
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		finalizedBlock := &types.BlockInfo{Height: finalizedHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return([]*types.BlockInfo{finalizedBlock}, nil).AnyTimes()
		mockClientController.EXPECT().QueryBlocks(finalizedHeight+1, currentHeight, uint64(10)).
			Return(catchUpBlocks, nil)
		mockClientController.EXPECT().SubmitBatchFinalitySigs(valIns.GetBtcPkBIP340(), catchUpBlocks, gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		result, err := valIns.FastSync(finalizedHeight+1, currentHeight)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedTxHash, result.Responses[0].TxHash)
		require.Equal(t, currentHeight, valIns.GetLastVotedHeight())
		require.Equal(t, currentHeight, valIns.GetLastProcessedHeight())
	})
}
