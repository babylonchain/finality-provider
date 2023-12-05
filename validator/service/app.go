package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	sdkmath "cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"go.uber.org/zap"

	valkr "github.com/babylonchain/btc-validator/keyring"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/client"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/babylonchain/btc-validator/validator/proto"

	valstore "github.com/babylonchain/btc-validator/validator/store"
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

	cc     clientcontroller.ClientController
	kr     keyring.Keyring
	vs     *valstore.ValidatorStore
	config *valcfg.Config
	logger *zap.Logger
	input  *strings.Reader

	validatorManager *ValidatorManager
	eotsManager      eotsmanager.EOTSManager

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	validatorRegisteredEventChan chan *validatorRegisteredEvent
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *zap.Logger,
) (*ValidatorApp, error) {
	cc, err := clientcontroller.NewClientController(config.ChainName, config.BabylonConfig, &config.ActiveNetParams, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client for the consumer chain %s: %v", config.ChainName, err)
	}

	// if the EOTSManagerAddress is empty, run a local EOTS manager;
	// otherwise connect a remote one with a gRPC client
	var em eotsmanager.EOTSManager
	if config.EOTSManagerAddress == "" {
		eotsCfg, err := valcfg.NewEOTSManagerConfigFromAppConfig(config)
		if err != nil {
			return nil, err
		}
		em, err = eotsmanager.NewLocalEOTSManager(eotsCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create EOTS manager locally: %w", err)
		}

		logger.Info("running EOTS manager locally")
	} else {
		em, err = client.NewEOTSManagerGRpcClient(config.EOTSManagerAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to create EOTS manager client: %w", err)
		}
		// TODO add retry mechanism and ping to ensure the EOTS manager daemon is healthy
		logger.Info("successfully connected to a remote EOTS manager", zap.String("address", config.EOTSManagerAddress))
	}

	return NewValidatorApp(config, cc, em, logger)
}

func NewValidatorApp(
	config *valcfg.Config,
	cc clientcontroller.ClientController,
	em eotsmanager.EOTSManager,
	logger *zap.Logger,
) (*ValidatorApp, error) {
	input := strings.NewReader("")
	kr, err := valkr.CreateKeyring(
		config.BabylonConfig.KeyDirectory,
		config.BabylonConfig.ChainID,
		config.BabylonConfig.KeyringBackend,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	valStore, err := valstore.NewValidatorStore(config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	vm, err := NewValidatorManager(valStore, config, cc, em, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator manager: %w", err)
	}

	return &ValidatorApp{
		cc:                           cc,
		vs:                           valStore,
		kr:                           kr,
		config:                       config,
		logger:                       logger,
		input:                        input,
		validatorManager:             vm,
		eotsManager:                  em,
		quit:                         make(chan struct{}),
		sentQuit:                     make(chan struct{}),
		eventQuit:                    make(chan struct{}),
		createValidatorRequestChan:   make(chan *createValidatorRequest),
		registerValidatorRequestChan: make(chan *registerValidatorRequest),
		validatorRegisteredEventChan: make(chan *validatorRegisteredEvent),
	}, nil
}

func (app *ValidatorApp) GetConfig() *valcfg.Config {
	return app.config
}

func (app *ValidatorApp) GetValidatorStore() *valstore.ValidatorStore {
	return app.vs
}

func (app *ValidatorApp) GetKeyring() keyring.Keyring {
	return app.kr
}

func (app *ValidatorApp) GetInput() *strings.Reader {
	return app.input
}

func (app *ValidatorApp) ListValidatorInstances() []*ValidatorInstance {
	return app.validatorManager.ListValidatorInstances()
}

// GetValidatorInstance returns the validator instance with the given Babylon public key
func (app *ValidatorApp) GetValidatorInstance(valPk *bbntypes.BIP340PubKey) (*ValidatorInstance, error) {
	return app.validatorManager.GetValidatorInstance(valPk)
}

func (app *ValidatorApp) RegisterValidator(valPkStr string) (*RegisterValidatorResponse, error) {
	valPk, err := bbntypes.NewBIP340PubKeyFromHex(valPkStr)
	if err != nil {
		return nil, err
	}

	validator, err := app.vs.GetStoreValidator(valPk.MustMarshal())
	if err != nil {
		return nil, err
	}

	if validator.Status != proto.ValidatorStatus_CREATED {
		return nil, fmt.Errorf("validator is already registered")
	}

	btcSig, err := bbntypes.NewBIP340Signature(validator.Pop.BtcSig)
	if err != nil {
		return nil, err
	}

	pop := &bstypes.ProofOfPossession{
		BabylonSig: validator.Pop.BabylonSig,
		BtcSig:     btcSig.MustMarshal(),
		BtcSigType: bstypes.BTCSigType_BIP340,
	}

	commissionRate, err := sdkmath.LegacyNewDecFromStr(validator.Commission)
	if err != nil {
		return nil, err
	}

	request := &registerValidatorRequest{
		bbnPubKey:       validator.GetBabylonPK(),
		btcPubKey:       validator.MustGetBIP340BTCPK(),
		pop:             pop,
		description:     validator.Description,
		commission:      &commissionRate,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *RegisterValidatorResponse, 1),
	}

	app.registerValidatorRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

// StartHandlingValidator starts a validator instance with the given Babylon public key
// Note: this should be called right after the validator is registered
func (app *ValidatorApp) StartHandlingValidator(valPk *bbntypes.BIP340PubKey, passphrase string) error {
	return app.validatorManager.StartValidator(valPk, passphrase)
}

// NOTE: this is not safe in production, so only used for testing purpose
func (app *ValidatorApp) getValPrivKey(valPk []byte) (*btcec.PrivateKey, error) {
	record, err := app.eotsManager.KeyRecord(valPk, "")
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
}

// Start starts only the validator daemon without any validator instances
func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Info("Starting ValidatorApp")

		app.eventWg.Add(1)
		go app.eventLoop()

		app.sentWg.Add(1)
		go app.registrationLoop()
	})

	return startErr
}

func (app *ValidatorApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Info("Stopping ValidatorApp")

		// Always stop the submission loop first to not generate additional events and actions
		app.logger.Debug("Stopping submission loop")
		close(app.quit)
		app.wg.Wait()

		app.logger.Debug("Stopping validators")
		if err := app.validatorManager.Stop(); err != nil {
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

		app.logger.Debug("Stopping EOTS manager")
		if err := app.eotsManager.Close(); err != nil {
			stopErr = err
			return
		}

		app.logger.Debug("ValidatorApp successfully stopped")

	})
	return stopErr
}

func (app *ValidatorApp) CreateValidator(
	keyName, chainID, passPhrase, hdPath string,
	description []byte,
	commission *sdkmath.LegacyDec,
) (*CreateValidatorResult, error) {

	req := &createValidatorRequest{
		keyName:         keyName,
		chainID:         chainID,
		passPhrase:      passPhrase,
		hdPath:          hdPath,
		description:     description,
		commission:      commission,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *createValidatorResponse, 1),
	}

	app.createValidatorRequestChan <- req

	select {
	case err := <-req.errResponse:
		return nil, err
	case successResponse := <-req.successResponse:
		return &CreateValidatorResult{
			ValPk: successResponse.ValPk,
		}, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

func (app *ValidatorApp) handleCreateValidatorRequest(req *createValidatorRequest) (*createValidatorResponse, error) {
	valPkBytes, err := app.eotsManager.CreateKey(req.keyName, req.passPhrase, req.hdPath)
	if err != nil {
		return nil, err
	}

	valPk, err := bbntypes.NewBIP340PubKey(valPkBytes)
	if err != nil {
		return nil, err
	}

	kr, err := valkr.NewChainKeyringControllerWithKeyring(app.kr, req.keyName, app.input)
	if err != nil {
		return nil, err
	}

	keyPair, err := kr.CreateChainKey(req.passPhrase, req.hdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain key for the validator: %w", err)
	}
	pk := &secp256k1.PubKey{Key: keyPair.PublicKey.SerializeCompressed()}

	valRecord, err := app.eotsManager.KeyRecord(valPk.MustMarshal(), req.passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator record: %w", err)
	}

	pop, err := kr.CreatePop(valRecord.PrivKey, req.passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create proof-of-possession of the validator: %w", err)
	}

	validator := valstore.NewStoreValidator(pk, valPk, req.keyName, req.chainID, pop, req.description, req.commission)

	if err := app.vs.SaveValidator(validator); err != nil {
		return nil, fmt.Errorf("failed to save validator: %w", err)
	}

	app.logger.Info("successfully created a validator",
		zap.String("btc_pk", valPk.MarshalHex()),
		zap.String("key_name", req.keyName),
	)

	return &createValidatorResponse{
		ValPk: valPk,
	}, nil
}

// main event loop for the validator app
func (app *ValidatorApp) eventLoop() {
	defer app.eventWg.Done()

	for {
		select {
		case req := <-app.createValidatorRequestChan:
			res, err := app.handleCreateValidatorRequest(req)
			if err != nil {
				req.errResponse <- err
				continue
			}

			req.successResponse <- &createValidatorResponse{ValPk: res.ValPk}

		case ev := <-app.validatorRegisteredEventChan:
			valStored, err := app.vs.GetStoreValidator(ev.btcPubKey.MustMarshal())
			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.Fatal(
					"registered validator not found in DB",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}

			// change the status of the validator to registered
			err = app.vs.SetValidatorStatus(valStored, proto.ValidatorStatus_REGISTERED)
			if err != nil {
				app.logger.Fatal("failed to set validator status to REGISTERED",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}

			// return to the caller
			ev.successResponse <- &RegisterValidatorResponse{
				bbnPubKey: valStored.GetBabylonPK(),
				btcPubKey: valStored.MustGetBIP340BTCPK(),
				TxHash:    ev.txHash,
			}

		case <-app.eventQuit:
			app.logger.Debug("exiting main event loop")
			return
		}
	}
}

func (app *ValidatorApp) registrationLoop() {
	defer app.sentWg.Done()
	for {
		select {
		case req := <-app.registerValidatorRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it fails due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			popBytes, err := req.pop.Marshal()
			if err != nil {
				req.errResponse <- err
				continue
			}

			res, err := app.cc.RegisterValidator(
				req.bbnPubKey.Key,
				req.btcPubKey.MustToBTCPK(),
				popBytes,
				req.commission,
				req.description,
			)

			if err != nil {
				app.logger.Error(
					"failed to register validator",
					zap.String("pk", req.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
				req.errResponse <- err
				continue
			}

			app.logger.Info(
				"successfully registered validator on babylon",
				zap.String("btc_pk", req.btcPubKey.MarshalHex()),
				zap.String("babylon_pk", hex.EncodeToString(req.bbnPubKey.Key)),
				zap.String("txHash", res.TxHash),
			)

			app.validatorRegisteredEventChan <- &validatorRegisteredEvent{
				btcPubKey: req.btcPubKey,
				bbnPubKey: req.bbnPubKey,
				txHash:    res.TxHash,
				// pass the channel to the event so that we can send the response to the user which requested
				// the registration
				successResponse: req.successResponse,
			}
		case <-app.sentQuit:
			app.logger.Debug("exiting registration loop")
			return
		}
	}
}
