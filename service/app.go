package service

import (
	"fmt"
	"sync"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/valcfg"

	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/val"
)

type ValidatorApp struct {
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	quit      chan struct{}

	bc     bbncli.BabylonClient
	vs     *val.ValidatorStore
	config *valcfg.Config
	logger *logrus.Logger
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
) (*ValidatorApp, error) {

	valStore, err := val.NewValidatorStore(config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	// TODO use real client
	babylonClient := &mocks.MockBabylonClient{}

	if err != nil {
		return nil, err
	}

	return &ValidatorApp{
		bc:     babylonClient,
		vs:     valStore,
		config: config,
		logger: logger,
		quit:   make(chan struct{}),
	}, nil
}

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		app.wg.Add(1)
		go app.eventLoop()
	})

	return startErr
}

func (app *ValidatorApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Infof("Stopping ValidatorApp")
		close(app.quit)
		app.wg.Wait()
	})
	return stopErr
}

// main event loop for the validator app
func (app *ValidatorApp) eventLoop() {
	panic("implement me")
}
