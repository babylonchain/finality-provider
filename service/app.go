package service

import (
	"fmt"
	"sync"

	"cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/client"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/types"
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

	cc     clientcontroller.ClientController
	kr     keyring.Keyring
	vs     *val.ValidatorStore
	config *valcfg.Config
	logger *logrus.Logger

	validatorManager *ValidatorManager
	eotsManager      eotsmanager.EOTSManager

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	validatorRegisteredEventChan chan *validatorRegisteredEvent
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
) (*ValidatorApp, error) {
	cc, err := clientcontroller.NewClientController(config, logger)
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
		logger.Infof("successfully connected to a remote EOTS manager at %s", config.EOTSManagerAddress)
	}

	return NewValidatorApp(config, cc, em, logger)
}

func NewValidatorApp(
	config *valcfg.Config,
	cc clientcontroller.ClientController,
	em eotsmanager.EOTSManager,
	logger *logrus.Logger,
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

	if config.JuryMode {
		kc, err := val.NewChainKeyringControllerWithKeyring(kr, config.JuryModeConfig.JuryKeyName)
		if err != nil {
			return nil, err
		}
		if _, err := kc.GetChainPrivKey(); err != nil {
			return nil, fmt.Errorf("the program is running in Jury mode but the Jury key %s is not found: %w",
				config.JuryModeConfig.JuryKeyName, err)
		}
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

func (app *ValidatorApp) GetValidatorStore() *val.ValidatorStore {
	return app.vs
}

func (app *ValidatorApp) GetKeyring() keyring.Keyring {
	return app.kr
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

	commissionRate, err := math.LegacyNewDecFromStr(validator.Commission)
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
func (app *ValidatorApp) StartHandlingValidator(valPk *bbntypes.BIP340PubKey) error {
	return app.validatorManager.addValidatorInstance(valPk)
}

func (app *ValidatorApp) StartHandlingValidators() error {
	return app.validatorManager.Start()
}

// AddJurySignature adds a Jury signature on the given Bitcoin delegation and submits it to Babylon
// Note: this should be only called when the program is running in Jury mode
func (app *ValidatorApp) AddJurySignature(btcDel *types.Delegation) (*AddJurySigResponse, error) {
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
		stakingTx.Script,
		juryPrivKey,
		&app.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	stakingTxHash := stakingMsgTx.TxHash().String()

	jurySchnorrSig, err := jurySig.ToBTCSig()
	if err != nil {
		return nil, err
	}
	res, err := app.cc.SubmitJurySig(btcDel.ValBtcPk, btcDel.BtcPk, stakingTxHash, jurySchnorrSig)

	valPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(btcDel.ValBtcPk).MarshalHex()
	delPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(btcDel.BtcPk).MarshalHex()
	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": valPkHex,
			"delBtcPubKey": delPkHex,
		}).Error("failed to submit Jury signature")
		return nil, err
	}

	if res == nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": valPkHex,
			"delBtcPubKey": delPkHex,
		}).Error("failed to submit Jury signature")
		return nil, fmt.Errorf("failed to submit Jury signature due to known error")
	}

	return &AddJurySigResponse{
		TxHash: res.TxHash,
	}, nil
}

// AddJurySignature adds a Jury signature on the given Bitcoin delegation and submits it to Babylon
// Note: this should be only called when the program is running in Jury mode
func (app *ValidatorApp) AddJuryUnbondingSignatures(del *types.Delegation) (*AddJurySigResponse, error) {
	if del == nil {
		return nil, fmt.Errorf("btc delegation is nil")
	}

	if del.BtcUndelegation == nil {
		return nil, fmt.Errorf("delegation does not have an unbonding transaction")
	}

	if del.BtcUndelegation.ValidatorUnbondingSig == nil {
		return nil, fmt.Errorf("delegation does not have a validator signature for unbonding transaction yet")
	}

	// In normal operation it is not possible to have one of this signatures and not have the other
	// as only way to update this fields in delegation is by processing the MsgAddJuryUnbondingSigs msg
	// which should update both fields at atomically in case of successfull transaction.
	if del.BtcUndelegation.JurySlashingSig != nil || del.BtcUndelegation.JuryUnbondingSig != nil {
		return nil, fmt.Errorf("delegation already has required jury signatures")
	}

	// get Jury private key from the keyring
	juryPrivKey, err := app.getJuryPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Jury private key: %w", err)
	}

	// 1. Sign unbonding transaction
	stakingTx := del.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	juryUnbondingSig, err := del.BtcUndelegation.UnbondingTx.Sign(
		stakingMsgTx,
		stakingTx.Script,
		juryPrivKey,
		&app.config.ActiveNetParams,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
	}

	// 2. Sign slash unbonding transaction
	slashUnbondigTx := del.BtcUndelegation.SlashingTx
	unbondingTx := del.BtcUndelegation.UnbondingTx
	unbondingMsgTx, err := unbondingTx.ToMsgTx()

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize unbonding tx: %w", err)
	}

	jurySlashingUnbondingSig, err := slashUnbondigTx.Sign(
		unbondingMsgTx,
		unbondingTx.Script,
		juryPrivKey,
		&app.config.ActiveNetParams,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign slash unbonding tx: %w", err)
	}

	stakingTxHash := stakingMsgTx.TxHash().String()

	juryUnbondingSchnorrSig, err := juryUnbondingSig.ToBTCSig()
	if err != nil {
		return nil, err
	}
	jurySlashingUnbondingShcnorrSig, err := jurySlashingUnbondingSig.ToBTCSig()
	if err != nil {
		return nil, err
	}
	res, err := app.cc.SubmitJuryUnbondingSigs(
		del.ValBtcPk,
		del.BtcPk,
		stakingTxHash,
		juryUnbondingSchnorrSig,
		jurySlashingUnbondingShcnorrSig,
	)

	valPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(del.ValBtcPk).MarshalHex()
	delPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(del.BtcPk).MarshalHex()

	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": valPkHex,
			"delBtcPubKey": delPkHex,
		}).Error("failed to submit Jury signature")
		return nil, err
	}

	if res == nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": valPkHex,
			"delBtcPubKey": delPkHex,
		}).Error("failed to submit Jury signature")
		return nil, fmt.Errorf("failed to submit Jury signature due to known error")
	}

	return &AddJurySigResponse{
		TxHash: res.TxHash,
	}, nil
}

func (app *ValidatorApp) getJuryPrivKey() (*btcec.PrivateKey, error) {
	kc, err := val.NewChainKeyringControllerWithKeyring(app.kr, app.config.JuryModeConfig.JuryKeyName)
	if err != nil {
		return nil, err
	}

	sdkPrivKey, err := kc.GetChainPrivKey()
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(sdkPrivKey.Key)

	return privKey, nil
}

func (app *ValidatorApp) getValPrivKey(valPk []byte) (*btcec.PrivateKey, error) {
	record, err := app.eotsManager.KeyRecord(valPk, "")
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
}

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		app.eventWg.Add(1)
		go app.eventLoop()

		app.sentWg.Add(1)
		go app.registrationLoop()

		if app.IsJury() {
			app.wg.Add(1)
			go app.jurySigSubmissionLoop()
		} else {
			if err := app.StartHandlingValidators(); err != nil {
				startErr = err
				return
			}
		}
	})

	return startErr
}

func (app *ValidatorApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Infof("Stopping ValidatorApp")

		// Always stop the submission loop first to not generate additional events and actions
		app.logger.Debug("Stopping submission loop")
		close(app.quit)
		app.wg.Wait()

		if !app.IsJury() {
			app.logger.Debug("Stopping validators")
			if err := app.validatorManager.Stop(); err != nil {
				stopErr = err
				return
			}
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

func (app *ValidatorApp) CreateValidator(keyName, chainID, passPhrase string, description *stakingtypes.Description, commission *sdktypes.Dec) (*CreateValidatorResult, error) {
	req := &createValidatorRequest{
		keyName:         keyName,
		chainID:         chainID,
		passPhrase:      passPhrase,
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

func (app *ValidatorApp) IsJury() bool {
	return app.config.JuryMode
}

func (app *ValidatorApp) handleCreateValidatorRequest(req *createValidatorRequest) (*createValidatorResponse, error) {
	valPkBytes, err := app.eotsManager.CreateKey(req.keyName, req.passPhrase)
	if err != nil {
		return nil, err
	}

	valPk, err := bbntypes.NewBIP340PubKey(valPkBytes)
	if err != nil {
		return nil, err
	}

	kr, err := val.NewChainKeyringControllerWithKeyring(app.kr, req.keyName)
	if err != nil {
		return nil, err
	}

	bbnPk, err := kr.CreateChainKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create chain key for the validator: %w", err)
	}

	valRecord, err := app.eotsManager.KeyRecord(valPk.MustMarshal(), req.passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator record: %w", err)
	}

	pop, err := kr.CreatePop(valRecord.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create proof-of-possession of the validator: %w", err)
	}

	validator := val.NewStoreValidator(bbnPk, valPk, req.keyName, req.chainID, pop, req.description, req.commission)

	if err := app.vs.SaveValidator(validator); err != nil {
		return nil, fmt.Errorf("failed to save validator: %w", err)
	}

	app.logger.WithFields(logrus.Fields{
		"btc_pub_key": valPk.MarshalHex(),
		"name":        req.keyName,
	}).Debug("successfully created a validator")

	return &createValidatorResponse{
		ValPk: valPk,
	}, nil
}
