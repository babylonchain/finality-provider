package testutil

import (
	"testing"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
)

func PrepareMockedClientController(t *testing.T, currentHeight uint64, lastCommitHash []byte) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockClientController := mocks.NewMockClientController(ctl)
	resHeader := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(currentHeight),
			LastCommitHash: lastCommitHash,
		},
	}

	mockClientController.EXPECT().QueryBestHeader().Return(resHeader, nil).AnyTimes()
	mockClientController.EXPECT().QueryLatestFinalizedBlocks(uint64(1)).Return(nil, nil)
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()

	return mockClientController
}
