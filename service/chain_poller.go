package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
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

func (cp *ChainPoller) latestBlockWithRetry() (*types.BlockInfo, error) {
	var (
		latestBlock *types.BlockInfo
		err         error
	)

	if err := retry.Do(func() error {
		latestBlock, err = cp.cc.QueryBestBlock()
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cp.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("failed to query the consumer chain for the latest height")
	})); err != nil {
		return nil, err
	}
	return latestBlock, nil
}

func (cp *ChainPoller) blockWithRetry(height uint64) (*types.BlockInfo, error) {
	var (
		block *types.BlockInfo
		err   error
	)
	if err := retry.Do(func() error {
		block, err = cp.cc.QueryBlock(height)
		if err != nil {
			return err
		}
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

	return block, nil
}

func (cp *ChainPoller) validateStartHeight(startHeight uint64) error {
	// Infinite retry to get initial latest height
	// TODO: Add possible cancellation or timeout for starting node

	if startHeight == 0 {
		return fmt.Errorf("start height can't be 0")
	}

	var currentBestChainHeight uint64
	for {
		lastestBlock, err := cp.latestBlockWithRetry()
		if err != nil {
			cp.logger.WithFields(logrus.Fields{
				"error": err,
			}).Error("Failed to query babylon for the latest status")
			continue
		}

		currentBestChainHeight = lastestBlock.Height
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
		blockToRetrieve := cp.GetNextHeight()
		block, err := cp.blockWithRetry(blockToRetrieve)
		if err != nil {
			failedCycles++

			cp.logger.WithFields(logrus.Fields{
				"error":        err,
				"currFailures": failedCycles,
				"blockToGet":   blockToRetrieve,
			}).Error("failed to query the consumer chain for the block")
		} else {
			// no error and we got the header we wanted to get, bump the state and push
			// notification about data
			cp.SetNextHeight(blockToRetrieve + 1)
			failedCycles = 0

			cp.logger.WithFields(logrus.Fields{
				"height": block.Height,
			}).Info("the poller retrieved the block from the consumer chain")

			// Push the data to the channel.
			// If the cosumers are to slow i.e the buffer is full, this will block and we will
			// stop retrieving data from the node.
			cp.blockInfoChan <- block
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
	return cp.getNextHeight()
}

func (cp *ChainPoller) getNextHeight() uint64 {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.nextHeight
}

func (cp *ChainPoller) setNextHeight(height uint64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.nextHeight = height
}

func (cp *ChainPoller) SetNextHeight(height uint64) {
	cp.setNextHeight(height)
}

func (cp *ChainPoller) SetNextHeightAndClearBuffer(height uint64) {
	cp.SetNextHeight(height)
	cp.clearChanBuffer()
}

func (cp *ChainPoller) clearChanBuffer() {
	for len(cp.blockInfoChan) > 0 {
		<-cp.blockInfoChan
	}
}
