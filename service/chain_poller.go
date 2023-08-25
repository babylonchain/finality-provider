package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	cfg "github.com/babylonchain/btc-validator/valcfg"
)

var (
	// TODO: Maybe configurable?
	RtyAttNum = uint(10)
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
	quit      chan struct{}

	bc            bbncli.BabylonClient
	cfg           *cfg.ChainPollerConfig
	blockInfoChan chan *BlockInfo
	logger        *logrus.Logger
}

type PollerState struct {
	BlockToRetrieve uint64
	FailedCycles    uint64
}

type BlockInfo struct {
	Height         uint64
	LastCommitHash []byte
	Finalized      bool
}

func NewChainPoller(
	logger *logrus.Logger,
	cfg *cfg.ChainPollerConfig,
	bc bbncli.BabylonClient,
) *ChainPoller {
	return &ChainPoller{
		logger:        logger,
		cfg:           cfg,
		bc:            bc,
		blockInfoChan: make(chan *BlockInfo, cfg.BufferSize),
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

		cp.wg.Add(1)
		go cp.pollChain(PollerState{
			BlockToRetrieve: startHeight,
			FailedCycles:    0,
		})
	})
	return startErr
}

func (cp *ChainPoller) Stop() error {
	var stopError error
	cp.stopOnce.Do(func() {
		cp.logger.Infof("Stopping the chain poller")
		err := cp.bc.Close()
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
func (cp *ChainPoller) GetBlockInfoChan() <-chan *BlockInfo {
	return cp.blockInfoChan
}

func (cp *ChainPoller) nodeStatusWithRetry() (*ctypes.ResultStatus, error) {
	var response *ctypes.ResultStatus
	if err := retry.Do(func() error {
		status, err := cp.bc.QueryNodeStatus()
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

func (cp *ChainPoller) blockWithRetry(height uint64) (*BlockInfo, error) {
	var (
		ib  *finalitytypes.IndexedBlock
		err error
	)

	if err = retry.Do(func() error {
		ib, err = cp.bc.QueryIndexedBlock(height)
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cp.logger.WithFields(logrus.Fields{
			"height":       height,
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("Failed to query Babylon indexed block")
	})); err != nil {
		return nil, err
	}
	return &BlockInfo{
		Height:         ib.Height,
		LastCommitHash: ib.LastCommitHash,
		Finalized:      ib.Finalized,
	}, nil
}

func (cp *ChainPoller) validateStartHeight(startHeight uint64) error {
	// Infinite retry to get initial latest height
	// TODO: Add possible cancellation or timeout for starting node
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

	if startHeight == 0 {
		return fmt.Errorf("start height can't be 0")
	}
	// Allow the start height to be the next chain height
	if startHeight > currentBestChainHeight+1 {
		return fmt.Errorf("start height %d is more than the next chain tip height %d", startHeight, currentBestChainHeight+1)
	}

	return nil
}

func (cp *ChainPoller) pollChain(initialState PollerState) {
	defer cp.wg.Done()

	var state = initialState

	for {
		// TODO: Handlig of request cancellation, as otherwise shutdown will be blocked
		// until request is finished
		block, err := cp.blockWithRetry(state.BlockToRetrieve)
		if err != nil {
			cp.logger.WithFields(logrus.Fields{
				"currFailures": state.FailedCycles,
				"headerToGet":  state.BlockToRetrieve,
				"error":        err,
			}).Error("Failed to query Babylon For the indexed block")

			state.FailedCycles = state.FailedCycles + 1
			if state.FailedCycles > maxFailedCycles {
				cp.logger.Fatal("Reached max failed cycles, exiting")
			}
		} else {
			// no error, bump the state and push notification about data
			state.BlockToRetrieve = state.BlockToRetrieve + 1
			state.FailedCycles = 0

			cp.logger.WithFields(logrus.Fields{
				"height": block.Height,
			}).Info("Retrieved indexed block from Babylon")

			// Push the data to the channel.
			// If the cosumers are to slow i.e the buffer is full, this will block and we will
			// stop retrieving data from the node.
			cp.blockInfoChan <- block
		}

		select {
		case <-time.After(cp.cfg.PollInterval):

		case <-cp.quit:
			cp.logger.Info("the poller is closing")
			return
		}
	}
}
