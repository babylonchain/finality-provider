package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/types"
	cfg "github.com/babylonchain/btc-validator/valcfg"
)

var (
	// TODO: Maybe configurable?
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

const (
	// TODO: Maybe configurable?
	maxFailedCycles = 20
)

type ChainPoller struct {
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	mu        sync.Mutex
	quit      chan struct{}

	cc            clientcontroller.ClientController
	cfg           *cfg.ChainPollerConfig
	blockInfoChan chan *types.BlockInfo
	nextHeight    uint64
	logger        *logrus.Logger
}

func NewChainPoller(
	logger *logrus.Logger,
	cfg *cfg.ChainPollerConfig,
	cc clientcontroller.ClientController,
) *ChainPoller {
	return &ChainPoller{
		logger:        logger,
		cfg:           cfg,
		cc:            cc,
		blockInfoChan: make(chan *types.BlockInfo, cfg.BufferSize),
		quit:          make(chan struct{}),
	}
}

func (cp *ChainPoller) Start(startHeight uint64) error {
	var startErr error
	cp.startOnce.Do(func() {
		cp.logger.Infof("Starting the chain poller")

		err := cp.validateStartHeight(startHeight)
		if err != nil {
			startErr = err
			return
		}

		cp.nextHeight = startHeight

		cp.wg.Add(1)

		go cp.pollChain()
	})
	return startErr
}

func (cp *ChainPoller) Stop() error {
	var stopError error
	cp.stopOnce.Do(func() {
		cp.logger.Infof("Stopping the chain poller")
		err := cp.cc.Close()
		if err != nil {
			stopError = err
			return
		}
		close(cp.quit)
		cp.wg.Wait()
	})

	return stopError
}

// Return read only channel for incoming blocks
// TODO: Handle the case when there is more than one consumer. Currently with more than
// one consumer blocks most probably will be received out of order to those consumers.
func (cp *ChainPoller) GetBlockInfoChan() <-chan *types.BlockInfo {
	return cp.blockInfoChan
}

func (cp *ChainPoller) nodeStatusWithRetry() (*ctypes.ResultStatus, error) {
	var response *ctypes.ResultStatus

	if err := retry.Do(func() error {
		status, err := cp.cc.QueryNodeStatus()
		if err != nil {
			return err
		}
		response = status
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cp.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("Failed to query babylon For the latest height")
	})); err != nil {
		return nil, err
	}
	return response, nil
}

func (cp *ChainPoller) headerWithRetry(height uint64) (*ctypes.ResultHeader, error) {
	var response *ctypes.ResultHeader
	if err := retry.Do(func() error {
		headerResult, err := cp.cc.QueryHeader(int64(height))
		if err != nil {
			return err
		}
		response = headerResult
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cp.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"height":       height,
			"error":        err,
		}).Debug("failed to query the consumer chain for the header")
	})); err != nil {
		return nil, err
	}

	return response, nil
}

func (cp *ChainPoller) validateStartHeight(startHeight uint64) error {
	// Infinite retry to get initial latest height
	// TODO: Add possible cancellation or timeout for starting node

	if startHeight == 0 {
		return fmt.Errorf("start height can't be 0")
	}

	var currentBestChainHeight uint64
	for {
		status, err := cp.nodeStatusWithRetry()
		if err != nil {
			cp.logger.WithFields(logrus.Fields{
				"error": err,
			}).Error("Failed to query babylon for the latest status")
			continue
		}

		currentBestChainHeight = uint64(status.SyncInfo.LatestBlockHeight)
		break
	}

	// Allow the start height to be the next chain height
	if startHeight > currentBestChainHeight+1 {
		return fmt.Errorf("start height %d is more than the next chain tip height %d", startHeight, currentBestChainHeight+1)
	}

	return nil
}

func (cp *ChainPoller) pollChain() {
	defer cp.wg.Done()

	var failedCycles uint64

	for {
		// TODO: Handlig of request cancellation, as otherwise shutdown will be blocked
		// until request is finished
		headerToRetrieve := cp.GetNextHeight()
		result, err := cp.headerWithRetry(headerToRetrieve)
		if err == nil && result.Header != nil {
			// no error and we got the header we wanted to get, bump the state and push
			// notification about data
			cp.SetNextHeight(headerToRetrieve + 1)
			failedCycles = 0

			cp.logger.WithFields(logrus.Fields{
				"height": result.Header.Height,
			}).Info("the poller retrieved the header from the consumer chain")

			// Push the data to the channel.
			// If the cosumers are to slow i.e the buffer is full, this will block and we will
			// stop retrieving data from the node.
			cp.blockInfoChan <- &types.BlockInfo{
				Height:         uint64(result.Header.Height),
				LastCommitHash: result.Header.LastCommitHash,
			}

		} else if err == nil && result.Header == nil {
			// no error but header is nil, means the header is not found on node
			failedCycles++

			cp.logger.WithFields(logrus.Fields{
				"error":        err,
				"currFailures": failedCycles,
				"headerToGet":  headerToRetrieve,
			}).Error("failed to retrieve header from the consumer chain. Header did not produced yet")

		} else {
			failedCycles++

			cp.logger.WithFields(logrus.Fields{
				"error":        err,
				"currFailures": failedCycles,
				"headerToGet":  headerToRetrieve,
			}).Error("failed to query the consumer chain for the header")
		}

		if failedCycles > maxFailedCycles {
			cp.logger.Fatal("the poller has reached the max failed cycles, exiting")
		}

		select {
		case <-time.After(cp.cfg.PollInterval):

		case <-cp.quit:
			return
		}
	}
}

func (cp *ChainPoller) GetNextHeight() uint64 {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.nextHeight
}

func (cp *ChainPoller) SetNextHeight(height uint64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.nextHeight = height
}
