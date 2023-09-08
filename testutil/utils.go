package testutil

import (
	"math/rand"
	"testing"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
)

func PrepareMockedClientController(t *testing.T, r *rand.Rand, finalizedHeight, currentHeight uint64) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockClientController := mocks.NewMockClientController(ctl)
	status := &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(currentHeight)},
	}

	for i := finalizedHeight; i <= currentHeight; i++ {
		resHeader := &coretypes.ResultHeader{
			Header: &cometbfttypes.Header{
				Height:         int64(currentHeight),
				LastCommitHash: GenRandomByteArray(r, 32),
			},
		}
		mockClientController.EXPECT().QueryHeader(int64(i)).Return(resHeader, nil).AnyTimes()
	}

	currentHeaderRes := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(currentHeight),
			LastCommitHash: GenRandomByteArray(r, 32),
		},
	}

	finalizedBlocks := make([]*types.BlockInfo, 0)
	finalizedBlock := &types.BlockInfo{
		Height:         finalizedHeight,
		LastCommitHash: GenRandomByteArray(r, 32),
	}
	finalizedBlocks = append(finalizedBlocks, finalizedBlock)

	mockClientController.EXPECT().QueryNodeStatus().Return(status, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(finalizedBlocks, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()
	mockClientController.EXPECT().QueryBestHeader().Return(currentHeaderRes, nil).AnyTimes()

	return mockClientController
}
