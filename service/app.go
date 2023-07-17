package service

import (
	"fmt"
	"sync"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/valcfg"

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
	bc bbncli.BabylonClient,
) (*ValidatorApp, error) {

	valStore, err := val.NewValidatorStore(config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	if err != nil {
		return nil, err
	}

	return &ValidatorApp{
		bc:     bc,
		vs:     valStore,
		config: config,
		logger: logger,
		quit:   make(chan struct{}),
	}, nil
}

func (app *ValidatorApp) GetValidatorStore() *val.ValidatorStore {
	return app.vs
}

func (app *ValidatorApp) RegisterValidator(pkBytes []byte) ([]byte, error) {
	validator, err := app.vs.GetValidator(pkBytes)
	if err != nil {
		return nil, err
	}

	// TODO: the following decoding is not needed if Babylon and cosmos protos are introduced

	bbnPk := &secp256k1.PubKey{Key: validator.BabylonPk}

	btcPk := new(types.BIP340PubKey)
	err = btcPk.Unmarshal(validator.BtcPk)
	if err != nil {
		return nil, err
	}

	btcSig, err := types.NewBIP340Signature(validator.Pop.BtcSig)
	if err != nil {
		return nil, err
	}

	pop := &bstypes.ProofOfPossession{
		BabylonSig: validator.Pop.BabylonSig,
		BtcSig:     btcSig,
	}

	return app.bc.RegisterValidator(bbnPk, btcPk, pop)
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
