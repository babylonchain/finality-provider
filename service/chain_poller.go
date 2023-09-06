package service

import (
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
	quit      chan struct{}

	cc        clientcontroller.ClientController
	cfg       *cfg.ChainPollerConfig
	blockChan chan *types.BlockInfo
	logger    *logrus.Logger
}

func NewChainPoller(
	logger *logrus.Logger,
	cfg *cfg.ChainPollerConfig,
	cc clientcontroller.ClientController,
) *ChainPoller {
	return &ChainPoller{
		logger:    logger,
		cfg:       cfg,
		cc:        cc,
		blockChan: make(chan *types.BlockInfo, cfg.BufferSize),
		quit:      make(chan struct{}),
	}
}

func (cp *ChainPoller) Start() error {
	var startErr error
	cp.startOnce.Do(func() {
		cp.logger.Infof("Starting the chain poller")

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
	return cp.blockChan
}

func (cp *ChainPoller) getBestBlock() (*types.BlockInfo, error) {
	var getBestBlock *types.BlockInfo
	if err := retry.Do(func() error {
		res, err := cp.cc.QueryBestHeader()
		if err != nil {
			return err
		}
		getBestBlock = &types.BlockInfo{
			Height:         uint64(res.Header.Height),
			LastCommitHash: res.Header.LastCommitHash,
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cp.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("Failed to query babylon For the latest header")
	})); err != nil {
		return nil, err
	}
	return getBestBlock, nil
}

func (cp *ChainPoller) pollChain() {
	defer cp.wg.Done()

	var failedCycles uint64
	for {
		// TODO: Handlig of request cancellation, as otherwise shutdown will be blocked
		// until request is finished
		b, err := cp.getBestBlock()
		if err == nil {
			// no error and we got the header we wanted to get, bump the state and push
			// notification about data
			failedCycles = 0

			cp.logger.WithFields(logrus.Fields{
				"height": b.Height,
			}).Info("retrieved the best block from the consumer chain")

			// Push the data to the channel.
			// If the cosumers are to slow i.e the buffer is full, this will block and we will
			// stop retrieving data from the node.
			cp.blockChan <- b
		} else {
			failedCycles += 1

			cp.logger.WithFields(logrus.Fields{
				"error":        err,
				"currFailures": failedCycles,
			}).Error("failed to query the consumer chain for the best header")
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
