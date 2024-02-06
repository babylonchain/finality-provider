package service_test

import (
	"math/rand"
	"sync"
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

// FuzzChainPoller_Start tests the poller polling blocks
// in sequence
func FuzzChainPoller_Start(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		currentHeight := uint64(r.Int63n(100) + 1)
		startHeight := currentHeight + 1
		endHeight := startHeight + uint64(r.Int63n(10)+1)

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

// FuzzChainPoller_SkipHeight tests the functionality of SkipHeight
func FuzzChainPoller_SkipHeight(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		currentHeight := uint64(r.Int63n(100) + 1)
		startHeight := currentHeight + 1
		endHeight := startHeight + uint64(r.Int63n(10)+1)
		skipHeight := endHeight + uint64(r.Int63n(10)+1)

		ctl := gomock.NewController(t)
		mockClientController := mocks.NewMockClientController(ctl)
		mockClientController.EXPECT().Close().Return(nil).AnyTimes()
		mockClientController.EXPECT().QueryActivatedHeight().Return(uint64(1), nil).AnyTimes()

		currentBlockRes := &types.BlockInfo{
			Height: currentHeight,
		}
		mockClientController.EXPECT().QueryBestBlock().Return(currentBlockRes, nil).AnyTimes()

		for i := startHeight; i <= skipHeight; i++ {
			resBlock := &types.BlockInfo{
				Height: i,
			}
			mockClientController.EXPECT().QueryBlock(i).Return(resBlock, nil).AnyTimes()
		}

		pollerCfg := fpcfg.DefaultChainPollerConfig()
		pollerCfg.PollInterval = 1 * time.Second
		poller := service.NewChainPoller(zap.NewNop(), &pollerCfg, mockClientController)
		err := poller.Start(startHeight)
		require.NoError(t, err)
		defer func() {
			err := poller.Stop()
			require.NoError(t, err)
		}()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wg.Done()
			// insert a skipToHeight request with height lower than the next
			// height to retrieve, expecting an error
			err = poller.SkipToHeight(poller.NextHeight() - 1)
			require.Error(t, err)
			// insert a skipToHeight request with a height higher than the
			// next height to retrieve
			err = poller.SkipToHeight(skipHeight)
			require.NoError(t, err)
		}()

		skipped := false
		for i := startHeight; i <= endHeight; i++ {
			if skipped {
				break
			}
			select {
			case info := <-poller.GetBlockInfoChan():
				if info.Height == skipHeight {
					skipped = true
				} else {
					require.Equal(t, i, info.Height)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("Failed to get block info")
			}
		}

		wg.Wait()

		require.Equal(t, skipHeight+1, poller.NextHeight())
	})
}
