package service_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/types"
)

// FuzzFastSync tests a case where we have voting power when the finality
// provider enters fast-sync.
// It is expected that the finality provider could catch up to the current
// height through fast-sync
func FuzzFastSync(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		randomRegiteredEpoch := uint64(r.Int63n(10) + 1)
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		finalizedHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		currentHeight := finalizedHeight + uint64(r.Int63n(10)+1)
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		// mock finalised BTC timestamped
		mockClientController.EXPECT().QueryLastFinalizedEpoch().Return(randomRegiteredEpoch, nil).AnyTimes()
		_, fpIns, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, mockClientController, randomStartingHeight, randomRegiteredEpoch)
		defer cleanUp()

		// mock voting power
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(fpIns.GetBtcPk(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()

		catchUpBlocks := testutil.GenBlocks(r, finalizedHeight+1, currentHeight)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		finalizedBlock := &types.BlockInfo{Height: finalizedHeight, Hash: testutil.GenRandomByteArray(r, 32)}
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return([]*types.BlockInfo{finalizedBlock}, nil).AnyTimes()
		mockClientController.EXPECT().QueryBlocks(finalizedHeight+1, currentHeight, uint64(10)).
			Return(catchUpBlocks, nil)
		mockClientController.EXPECT().SubmitBatchFinalitySigs(fpIns.GetBtcPk(), catchUpBlocks, gomock.Any()).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		result, err := fpIns.FastSync(finalizedHeight+1, currentHeight)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedTxHash, result.Responses[0].TxHash)
		require.Equal(t, currentHeight, fpIns.GetLastVotedHeight())
		require.Equal(t, currentHeight, fpIns.GetLastProcessedHeight())
	})
}
