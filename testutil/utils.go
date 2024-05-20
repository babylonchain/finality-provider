package testutil

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/finality-provider/testutil/mocks"
	"github.com/babylonchain/finality-provider/types"
)

const TestPubRandNum = 25

func ZeroCommissionRate() *sdkmath.LegacyDec {
	zeroCom := sdkmath.LegacyZeroDec()
	return &zeroCom
}

func PrepareMockedConsumerController(t *testing.T, r *rand.Rand, startHeight, currentHeight uint64) *mocks.MockConsumerController {
	ctl := gomock.NewController(t)
	mockConsumerController := mocks.NewMockConsumerController(ctl)

	for i := startHeight + 1; i <= currentHeight; i++ {
		resBlock := &types.BlockInfo{
			Height: currentHeight,
			Hash:   GenRandomByteArray(r, 32),
		}
		mockConsumerController.EXPECT().QueryBlock(i).Return(resBlock, nil).AnyTimes()
	}

	mockConsumerController.EXPECT().Close().Return(nil).AnyTimes()
	mockConsumerController.EXPECT().QueryLatestBlockHeight().Return(currentHeight, nil).AnyTimes()
	mockConsumerController.EXPECT().QueryActivatedHeight().Return(uint64(1), nil).AnyTimes()

	return mockConsumerController
}

func PrepareMockedBabylonController(t *testing.T, randomRegiteredEpoch uint64) *mocks.MockClientController {
	ctl := gomock.NewController(t)
	mockBabylonController := mocks.NewMockClientController(ctl)
	// mock finalised BTC timestamped
	mockBabylonController.EXPECT().QueryLastFinalizedEpoch().Return(randomRegiteredEpoch, nil).AnyTimes()
	mockBabylonController.EXPECT().Close().Return(nil).AnyTimes()

	return mockBabylonController
}
