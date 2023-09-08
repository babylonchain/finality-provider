package testutil

import (
	"testing"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
)

func PrepareMockedClientController(t *testing.T, blockHeight uint64, lastCommitHash []byte) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockClientController := mocks.NewMockClientController(ctl)
	status := &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(blockHeight + 1)},
	}
	resHeader := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(blockHeight),
			LastCommitHash: lastCommitHash,
		},
	}

	finalizedBlocks := make([]*types.BlockInfo, 0)
	finalizedBlock := &types.BlockInfo{
		Height:         blockHeight,
		LastCommitHash: lastCommitHash,
	}
	finalizedBlocks = append(finalizedBlocks, finalizedBlock)

	mockClientController.EXPECT().QueryNodeStatus().Return(status, nil).AnyTimes()
	mockClientController.EXPECT().QueryHeader(int64(blockHeight)).Return(resHeader, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(finalizedBlocks, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()
	mockClientController.EXPECT().QueryBestHeader().Return(resHeader, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()

	return mockClientController
}
