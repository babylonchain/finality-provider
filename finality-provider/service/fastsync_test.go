package service_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/types"
)

func FuzzFastSync(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomStartingHeight := uint64(r.Int63n(100) + 1)
		finalizedHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		currentHeight := finalizedHeight + uint64(r.Int63n(10)+1)
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, Hash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		app, storeFp, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()
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

		// fast sync
		catchUpBlocks := testutil.GenBlocks(r, finalizedHeight+1, currentHeight)
		expectedTxHash = testutil.GenRandomHexStr(r, 32)
		finalizedBlock := &types.BlockInfo{Height: finalizedHeight, Hash: testutil.GenRandomByteArray(r, 32)}
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return([]*types.BlockInfo{finalizedBlock}, nil).AnyTimes()
		mockClientController.EXPECT().QueryBlocks(finalizedHeight+1, currentHeight, uint64(10)).
			Return(catchUpBlocks, nil)
		mockClientController.EXPECT().SubmitBatchFinalitySigs(fpIns.MustGetBtcPk(), catchUpBlocks, gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		result, err := fpIns.FastSync(finalizedHeight+1, currentHeight)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedTxHash, result.Responses[0].TxHash)
		require.Equal(t, currentHeight, fpIns.GetLastVotedHeight())
		require.Equal(t, currentHeight, fpIns.GetLastProcessedHeight())
	})
}
