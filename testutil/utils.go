package testutil

import (
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
)

func PrepareMockedClientController(t *testing.T, blockHeight uint64, lastCommitHash []byte) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockClientController := mocks.NewMockClientController(ctl)
	blocks := make([]*types.BlockInfo, 0)
	bi := &types.BlockInfo{
		Height:         blockHeight,
		LastCommitHash: lastCommitHash,
	}
	blocks = append(blocks, bi)

	mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(blocks, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestUnfinalizedBlocks(uint64(1)).Return(blocks, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()

	return mockClientController
}
