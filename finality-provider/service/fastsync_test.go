package service_test

import (
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
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
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight)
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(nil, nil).AnyTimes()
		_, fpIns, cleanUp := startFinalityProviderAppWithRegisteredFp(t, r, mockClientController, randomStartingHeight)
		defer cleanUp()

		mockClientController.EXPECT().QueryFinalityProviderVotingPower(fpIns.MustGetBtcPk(), gomock.Any()).
			Return(uint64(1), nil).AnyTimes()
		lastCommittedHeight := randomStartingHeight + 25
		lastCommittedPubRandMap := make(map[uint64]*btcec.FieldVal)
		lastCommittedPubRandMap[lastCommittedHeight] = testutil.GenPublicRand(r, t).ToFieldVal()
		mockClientController.EXPECT().QueryLastCommittedPublicRand(gomock.Any(), uint64(1)).Return(lastCommittedPubRandMap, nil).AnyTimes()

		// fast sync
		catchUpBlocks := testutil.GenBlocks(r, finalizedHeight+1, currentHeight)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
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
