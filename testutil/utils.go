package testutil

import (
	"testing"

	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
)

func PrepareMockedBabylonClient(t *testing.T, blockHeight uint64, lastCommitHash []byte) *mocks.MockBabylonClient {
	ctl := gomock.NewController(t)
	mockBabylonClient := mocks.NewMockBabylonClient(ctl)
	status := &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(blockHeight + 1)},
	}
	resHeader := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(blockHeight),
			LastCommitHash: lastCommitHash,
		},
	}
	finalizedBlocks := make([]*finalitytypes.IndexedBlock, 0)
	finalizedBlock := &finalitytypes.IndexedBlock{
		Height:         blockHeight,
		LastCommitHash: lastCommitHash,
		Finalized:      true,
	}
	finalizedBlocks = append(finalizedBlocks, finalizedBlock)

	mockBabylonClient.EXPECT().QueryNodeStatus().Return(status, nil).AnyTimes()
	mockBabylonClient.EXPECT().QueryHeader(int64(blockHeight)).Return(resHeader, nil).AnyTimes()
	mockBabylonClient.EXPECT().QueryLatestFinalisedBlocks(uint64(1)).Return(finalizedBlocks, nil).AnyTimes()
	mockBabylonClient.EXPECT().Close().Return(nil).AnyTimes()

	return mockBabylonClient
}
