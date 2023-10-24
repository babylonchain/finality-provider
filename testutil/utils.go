package testutil

import (
	"math/rand"
	"testing"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"

	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
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

	for i := startHeight + 1; i <= currentHeight; i++ {
		resBlock := &types.BlockInfo{
			Height:         currentHeight,
			LastCommitHash: GenRandomByteArray(r, 32),
		}
		mockClientController.EXPECT().QueryBlock(i).Return(resBlock, nil).AnyTimes()
	}

	currentBlockRes := &types.BlockInfo{
		Height:         currentHeight,
		LastCommitHash: GenRandomByteArray(r, 32),
	}

	mockClientController.EXPECT().Close().Return(nil).AnyTimes()
	mockClientController.EXPECT().QueryBestBlock().Return(currentBlockRes, nil).AnyTimes()

	return mockClientController
}
