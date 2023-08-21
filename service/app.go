package service

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/valcfg"

	"github.com/babylonchain/btc-validator/val"
)

type ValidatorApp struct {
	startOnce sync.Once
	stopOnce  sync.Once

	wg   sync.WaitGroup
	quit chan struct{}

	sentWg   sync.WaitGroup
	sentQuit chan struct{}

	eventWg   sync.WaitGroup
	eventQuit chan struct{}

	bc     bbncli.BabylonClient
	kr     keyring.Keyring
	vs     *val.ValidatorStore
	config *valcfg.Config
	logger *logrus.Logger
	poller *ChainPoller

	validatorManager *ValidatorManager

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	addJurySigRequestChan        chan *addJurySigRequest

	validatorRegisteredEventChan chan *validatorRegisteredEvent
	jurySigAddedEventChan        chan *jurySigAddedEvent
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
	bc bbncli.BabylonClient,
) (*ValidatorApp, error) {

	kr, err := CreateKeyring(config.BabylonConfig.KeyDirectory,
		config.BabylonConfig.ChainID,
		config.BabylonConfig.KeyringBackend)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	valStore, err := val.NewValidatorStore(config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	poller := NewChainPoller(logger, config.PollerConfig, bc)

	if config.JuryMode {
		if _, err := kr.Key(config.JuryModeConfig.JuryKeyName); err != nil {
			return nil, fmt.Errorf("the program is running in Jury mode but the Jury key %s is not found: %w",
				config.JuryModeConfig.JuryKeyName, err)
		}
	}

	return &ValidatorApp{
		bc:                           bc,
		vs:                           valStore,
		kr:                           kr,
		config:                       config,
		logger:                       logger,
		poller:                       poller,
		validatorManager:             NewValidatorManager(),
		quit:                         make(chan struct{}),
		sentQuit:                     make(chan struct{}),
		eventQuit:                    make(chan struct{}),
		createValidatorRequestChan:   make(chan *createValidatorRequest),
		registerValidatorRequestChan: make(chan *registerValidatorRequest),
		addJurySigRequestChan:        make(chan *addJurySigRequest),
		validatorRegisteredEventChan: make(chan *validatorRegisteredEvent),
		jurySigAddedEventChan:        make(chan *jurySigAddedEvent),
	}, nil
}

func (app *ValidatorApp) GetConfig() *valcfg.Config {
	return app.config
}

func (app *ValidatorApp) GetValidatorStore() *val.ValidatorStore {
	return app.vs
}

func (app *ValidatorApp) GetKeyring() keyring.Keyring {
	return app.kr
}

func (app *ValidatorApp) GetJuryPk() (*btcec.PublicKey, error) {
	juryPrivKey, err := app.getJuryPrivKey()
	if err != nil {
		return nil, err
	}
	return juryPrivKey.PubKey(), nil
}

func (app *ValidatorApp) ListValidatorInstances() []*ValidatorInstance {
	return app.validatorManager.listValidatorInstances()
}

// GetValidatorInstance returns the validator instance with the given Babylon public key
func (app *ValidatorApp) GetValidatorInstance(babylonPk *secp256k1.PubKey) (*ValidatorInstance, error) {
	return app.validatorManager.getValidatorInstance(babylonPk)
}

func (app *ValidatorApp) GetCurrentBbnBlock() (*BlockInfo, error) {
	header, err := app.bc.QueryBestHeader()
	if err != nil {
		return nil, err
	}

	return &BlockInfo{
		Height:         uint64(header.Header.Height),
		LastCommitHash: header.Header.LastCommitHash,
	}, nil
}

func (app *ValidatorApp) RegisterValidator(keyName string) (*RegisterValidatorResponse, *secp256k1.PubKey, error) {
	kc, err := val.NewKeyringControllerWithKeyring(app.kr, keyName)
	if err != nil {
		return nil, nil, err
	}
	if !kc.ValidatorKeyExists() {
		return nil, nil, fmt.Errorf("key name %s does not exist", keyName)
	}
	babylonPublicKeyBytes, err := kc.GetBabylonPublicKeyBytes()
	if err != nil {
		return nil, nil, err
	}
	validator, err := app.vs.GetStoreValidator(babylonPublicKeyBytes)
	if err != nil {
		return nil, nil, err
	}

	if validator.Status != proto.ValidatorStatus_CREATED {
		return nil, nil, fmt.Errorf("validator is already registered")
	}

	btcSig, err := types.NewBIP340Signature(validator.Pop.BtcSig)
	if err != nil {
		return nil, nil, err
	}

	pop := &bstypes.ProofOfPossession{
		BabylonSig: validator.Pop.BabylonSig,
		BtcSig:     btcSig,
	}

	request := &registerValidatorRequest{
		bbnPubKey:       validator.GetBabylonPK(),
		btcPubKey:       validator.MustGetBIP340BTCPK(),
		pop:             pop,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *RegisterValidatorResponse, 1),
	}

	app.registerValidatorRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, nil, err
	case successResponse := <-request.successResponse:
		return successResponse, validator.GetBabylonPK(), nil
	case <-app.quit:
		return nil, nil, fmt.Errorf("validator app is shutting down")
	}
}

// StartHandlingValidator starts a validator instance with the given Babylon public key
// Note: this should be called right after the validator is registered
func (app *ValidatorApp) StartHandlingValidator(bbnPk *secp256k1.PubKey) error {
	return app.validatorManager.addValidatorInstance(bbnPk, app.config, app.vs, app.kr, app.bc, app.logger)
}

func (app *ValidatorApp) StartHandlingValidators() error {
	storedValidators, err := app.vs.ListRegisteredValidators()
	if err != nil {
		return err
	}

	for _, v := range storedValidators {
		err = app.StartHandlingValidator(v.GetBabylonPK())
		if err != nil {
			return err
		}
	}

	return nil
}

// AddJurySignature adds a Jury signature on the given Bitcoin delegation and submits it to Babylon
// Note: this should be only called when the program is running in Jury mode
func (app *ValidatorApp) AddJurySignature(btcDel *bstypes.BTCDelegation) (*AddJurySigResponse, error) {
	if btcDel.JurySig != nil {
		return nil, fmt.Errorf("the Jury sig already existed in the Bitcoin delection")
	}

	slashingTx := btcDel.SlashingTx
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()
	if err != nil {
		return nil, err
	}

	// get Jury private key from the keyring
	juryPrivKey, err := app.getJuryPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Jury private key: %w", err)
	}

	jurySig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		juryPrivKey,
		&app.config.JuryModeConfig.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	request := &addJurySigRequest{
		bbnPubKey:       btcDel.BabylonPk,
		valBtcPk:        btcDel.ValBtcPk,
		delBtcPk:        btcDel.BtcPk,
		stakingTxHash:   stakingMsgTx.TxHash().String(),
		sig:             jurySig,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *AddJurySigResponse, 1),
	}

	app.addJurySigRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

func (app *ValidatorApp) getJuryPrivKey() (*btcec.PrivateKey, error) {
	return app.getPrivKey(app.config.JuryModeConfig.JuryKeyName)
}

func (app *ValidatorApp) getBtcPrivKey(name string) (*btcec.PrivateKey, error) {
	return app.getPrivKey(val.KeyName(name).GetBtcKeyName())
}

func (app *ValidatorApp) getPrivKey(name string) (*btcec.PrivateKey, error) {
	k, err := app.kr.Key(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s from the keyring: %w", name, err)
	}
	localKey := k.GetLocal().PrivKey.GetCachedValue()
	switch v := localKey.(type) {
	case *secp256k1.PrivKey:
		privKey, _ := btcec.PrivKeyFromBytes(v.Key)
		return privKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}
}

func (app *ValidatorApp) latestFinalisedBlocksWithRetry(count uint64) ([]*ftypes.IndexedBlock, error) {
	var response []*ftypes.IndexedBlock
	if err := retry.Do(func() error {
		latestFinalisedBlock, err := app.bc.QueryLatestFinalisedBlocks(count)
		if err != nil {
			return err
		}
		response = latestFinalisedBlock
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		app.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("Failed to query babylon for the latest finalised blocks")
	})); err != nil {
		return nil, err
	}
	return response, nil

}

func (app *ValidatorApp) getPollerStartingHeight() (uint64, error) {
	if !app.config.ValidatorModeConfig.AutoChainScanningMode {
		return app.config.ValidatorModeConfig.StaticChainScanningStartHeight, nil
	}
	earliestVotedHeight, err := app.vs.GetEarliestActiveValidatorVotedHeight()
	if err != nil {
		return 0, err
	}

	// Set initial block to the maximum of
	//    - earliestVotedHeight
	//    - the latest Babylon finalised block
	// The above is to ensure that:
	//
	//	(1) Any validator that is eligible to vote for a block,
	//	 doesn't miss submitting a vote for it.
	//	(2) The validators do not submit signatures for any already
	//	 finalised blocks.
	var initialBlockToGet uint64
	latestFinalisedBlock, err := app.latestFinalisedBlocksWithRetry(1)
	if err != nil {
		return 0, err
	}
	if len(latestFinalisedBlock) != 0 {
		if earliestVotedHeight > latestFinalisedBlock[0].Height {
			initialBlockToGet = earliestVotedHeight
		} else {
			initialBlockToGet = latestFinalisedBlock[0].Height
		}
	} else {
		initialBlockToGet = earliestVotedHeight
	}

	// ensure that initialBlockToGet is at least 1
	if initialBlockToGet == 0 {
		initialBlockToGet = 1
	}
	return initialBlockToGet, nil
}

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		// We perform this calculation here as we do not want to expose the database
		// to the poller.
		startHeight, err := app.getPollerStartingHeight()
		if err != nil {
			startErr = err
			return
		}

		if err := app.poller.Start(startHeight); err != nil {
			startErr = err
			return
		}

		app.eventWg.Add(1)
		go app.eventLoop()

		app.sentWg.Add(1)
		go app.handleSentToBabylonLoop()

		// Start submission loop last, as at this point both eventLoop and sentToBabylonLoop
		// are already running
		app.wg.Add(1)
		if app.IsJury() {
			go app.jurySigSubmissionLoop()
		} else {
			if err := app.StartHandlingValidators(); err != nil {
				startErr = err
				return
			}
			go app.validatorSubmissionLoop()
		}
	})

	return startErr
}

func (app *ValidatorApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Infof("Stopping ValidatorApp")
		if err := app.poller.Stop(); err != nil {
			stopErr = err
			return
		}

		// Always stop the submission loop first to not generate addional events and actions
		app.logger.Debug("Stopping submission loop")
		close(app.quit)
		app.wg.Wait()

		app.logger.Debug("Stopping validators")
		if err := app.validatorManager.stop(); err != nil {
			stopErr = err
			return
		}

		app.logger.Debug("Sent to Babylon loop stopped")
		close(app.sentQuit)
		app.sentWg.Wait()

		app.logger.Debug("Stopping main eventLoop")
		close(app.eventQuit)
		app.eventWg.Wait()

		// Closing db as last to avoid anybody to write do db
		app.logger.Debug("Stopping data store")
		if err := app.vs.Close(); err != nil {
			stopErr = err
			return
		}

		app.logger.Debug("ValidatorApp successfuly stopped")

	})
	return stopErr
}

func (app *ValidatorApp) CreateValidator(keyName string) (*CreateValidatorResult, error) {
	req := &createValidatorRequest{
		keyName:         keyName,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *createValidatorResponse, 1),
	}

	app.createValidatorRequestChan <- req

	select {
	case err := <-req.errResponse:
		return nil, err
	case successResponse := <-req.successResponse:
		return &CreateValidatorResult{
			BtcValidatorPk:     successResponse.BtcValidatorPk,
			BabylonValidatorPk: successResponse.BabylonValidatorPk,
		}, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

func (app *ValidatorApp) IsJury() bool {
	return app.config.JuryMode
}

func (app *ValidatorApp) handleCreateValidatorRequest(req *createValidatorRequest) (*createValidatorResponse, error) {

	app.logger.Debug("handling CreateValidator request")

	kr, err := val.NewKeyringControllerWithKeyring(app.kr, req.keyName)

	if err != nil {
		return nil, fmt.Errorf("failed to create keyring controller: %w", err)
	}

	if kr.ValidatorKeyNameTaken() {
		return nil, fmt.Errorf("the key name %s is taken", kr.GetKeyName())
	}

	// TODO should not expose direct proto here, as this is internal db representation
	// connected to serialization
	validator, err := kr.CreateBTCValidator()
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	if err := app.vs.SaveValidator(validator); err != nil {
		return nil, fmt.Errorf("failed to save validator: %w", err)
	}

	btcPubKey := validator.MustGetBTCPK()
	babylonPubKey := validator.GetBabylonPK()

	app.logger.Info("successfully created validator")
	app.logger.WithFields(logrus.Fields{
		"btc_pub_key":     hex.EncodeToString(btcPubKey.SerializeCompressed()),
		"babylon_pub_key": hex.EncodeToString(babylonPubKey.Key),
	}).Debug("created validator")

	return &createValidatorResponse{
		BtcValidatorPk:     *btcPubKey,
		BabylonValidatorPk: *babylonPubKey,
	}, nil
}
