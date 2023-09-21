package testutil

import (
	"math/rand"
	"testing"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
)

func EmptyDescription() *stakingtypes.Description {
	return &stakingtypes.Description{}
}

func ZeroCommissionRate() *sdktypes.Dec {
	zeroCom := sdktypes.ZeroDec()
	return &zeroCom
}

func PrepareMockedClientController(t *testing.T, r *rand.Rand, startHeight, currentHeight uint64) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockClientController := mocks.NewMockClientController(ctl)
	status := &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(currentHeight)},
	}

	for i := startHeight + 1; i <= currentHeight; i++ {
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

	mockClientController.EXPECT().QueryNodeStatus().Return(status, nil).AnyTimes()
	mockClientController.EXPECT().Close().Return(nil).AnyTimes()
	mockClientController.EXPECT().QueryBestHeader().Return(currentHeaderRes, nil).AnyTimes()

	return mockClientController
}
