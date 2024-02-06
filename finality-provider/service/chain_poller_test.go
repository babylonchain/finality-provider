package service_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/testutil/mocks"
	"github.com/babylonchain/finality-provider/types"
)

func FuzzChainPoller_Start(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		currentHeight := uint64(r.Int63n(100) + 1)
		startHeight := currentHeight + 1
		ranHeights := uint64(r.Int63n(10) + 1)
		endHeight := startHeight + ranHeights

		ctl := gomock.NewController(t)
		mockClientController := mocks.NewMockClientController(ctl)
		mockClientController.EXPECT().Close().Return(nil).AnyTimes()
		mockClientController.EXPECT().QueryActivatedHeight().Return(uint64(1), nil).AnyTimes()

		currentBlockRes := &types.BlockInfo{
			Height: currentHeight,
		}
		mockClientController.EXPECT().QueryBestBlock().Return(currentBlockRes, nil).AnyTimes()

		for i := startHeight; i <= endHeight; i++ {
			resBlock := &types.BlockInfo{
				Height: i,
			}
			mockClientController.EXPECT().QueryBlock(i).Return(resBlock, nil).AnyTimes()
		}

		pollerCfg := fpcfg.DefaultChainPollerConfig()
		pollerCfg.PollInterval = 10 * time.Millisecond
		poller := service.NewChainPoller(zap.NewNop(), &pollerCfg, mockClientController)
		err := poller.Start(startHeight)
		require.NoError(t, err)
		defer func() {
			err := poller.Stop()
			require.NoError(t, err)
		}()

		for i := startHeight; i <= endHeight; i++ {
			select {
			case info := <-poller.GetBlockInfoChan():
				require.Equal(t, i, info.Height)
			case <-time.After(10 * time.Second):
				t.Fatalf("Failed to get block info")
			}
		}
	})
}
