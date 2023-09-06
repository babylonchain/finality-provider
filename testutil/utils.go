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
	resHeader := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(blockHeight),
			LastCommitHash: lastCommitHash,
		},
	}
	blocks := make([]*types.BlockInfo, 0)
	bi := &types.BlockInfo{
		Height:         blockHeight,
		LastCommitHash: lastCommitHash,
	}
	blocks = append(blocks, bi)

	mockClientController.EXPECT().QueryBestHeader().Return(resHeader, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(blocks, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestUnfinalizedBlocks(uint64(1)).Return(blocks, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()

	return mockClientController
}
