package service

import (
	"encoding/hex"
	"fmt"
	"sync"

	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/clientcontroller"
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

	cc     clientcontroller.ClientController
	kr     keyring.Keyring
	vs     *val.ValidatorStore
	config *valcfg.Config
	logger *logrus.Logger

	validatorManager *ValidatorManager

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	validatorRegisteredEventChan chan *validatorRegisteredEvent
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
	cc clientcontroller.ClientController,
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
		if _, err := kr.Key(config.JuryModeConfig.JuryKeyName); err != nil {
			return nil, fmt.Errorf("the program is running in Jury mode but the Jury key %s is not found: %w",
				config.JuryModeConfig.JuryKeyName, err)
		}
	}

	return &ValidatorApp{
		cc:                           cc,
		vs:                           valStore,
		kr:                           kr,
		config:                       config,
		logger:                       logger,
		validatorManager:             NewValidatorManager(),
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

	btcSig, err := bbntypes.NewBIP340Signature(validator.Pop.BtcSig)
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
	return app.validatorManager.addValidatorInstance(bbnPk, app.config, app.vs, app.kr, app.cc, app.logger)
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
		stakingTx.Script,
		juryPrivKey,
		&app.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	stakingTxHash := stakingMsgTx.TxHash().String()

	res, err := app.cc.SubmitJurySig(btcDel.ValBtcPk, btcDel.BtcPk, stakingTxHash, jurySig)

	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": btcDel.ValBtcPk.MarshalHex(),
			"delBtcPubKey": btcDel.BtcPk.MarshalHex(),
		}).Error("failed to submit Jury signature")
		return nil, err
	}

	if res == nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": btcDel.ValBtcPk.MarshalHex(),
			"delBtcPubKey": btcDel.BtcPk.MarshalHex(),
		}).Error("failed to submit Jury signature")
		return nil, fmt.Errorf("failed to submit Jury signature due to known error")
	}

	return &AddJurySigResponse{
		TxHash: res.TxHash,
	}, nil
}

// AddJurySignature adds a Jury signature on the given Bitcoin delegation and submits it to Babylon
// Note: this should be only called when the program is running in Jury mode
func (app *ValidatorApp) AddJuryUnbondingSignatures(btcDel *bstypes.BTCDelegation) (*AddJurySigResponse, error) {
	if btcDel == nil {
		return nil, fmt.Errorf("btc delegation is nil")
	}

	if btcDel.BtcUndelegation == nil {
		return nil, fmt.Errorf("delegation does not have an unbonding transaction")
	}

	if btcDel.BtcUndelegation.ValidatorUnbondingSig == nil {
		return nil, fmt.Errorf("delegation does not have a validator signature for unbonding transaction yet")
	}

	// In normal operation it is not possible to have one of this signatures and not have the other
	// as only way to update this fields in delegation is by processing the MsgAddJuryUnbondingSigs msg
	// which should update both fields at atomically in case of successfull transaction.
	if btcDel.BtcUndelegation.JurySlashingSig != nil || btcDel.BtcUndelegation.JuryUnbondingSig != nil {
		return nil, fmt.Errorf("delegation already has required jury signatures")
	}

	// get Jury private key from the keyring
	juryPrivKey, err := app.getJuryPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Jury private key: %w", err)
	}

	// 1. Sign unbonding transaction
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	juryUnbondingSig, err := btcDel.BtcUndelegation.UnbondingTx.Sign(
		stakingMsgTx,
		stakingTx.Script,
		juryPrivKey,
		&app.config.ActiveNetParams,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
	}

	// 2. Sign slash unbonding transaction
	slashUnbondigTx := btcDel.BtcUndelegation.SlashingTx
	unbondingTx := btcDel.BtcUndelegation.UnbondingTx
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

	res, err := app.cc.SubmitJuryUnbondingSigs(
		btcDel.ValBtcPk,
		btcDel.BtcPk,
		stakingTxHash,
		juryUnbondingSig,
		jurySlashingUnbondingSig,
	)

	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": btcDel.ValBtcPk.MarshalHex(),
			"delBtcPubKey": btcDel.BtcPk.MarshalHex(),
		}).Error("failed to submit Jury signature")
		return nil, err
	}

	if res == nil {
		app.logger.WithFields(logrus.Fields{
			"err":          err,
			"valBtcPubKey": btcDel.ValBtcPk.MarshalHex(),
			"delBtcPubKey": btcDel.BtcPk.MarshalHex(),
		}).Error("failed to submit Jury signature")
		return nil, fmt.Errorf("failed to submit Jury signature due to known error")
	}

	return &AddJurySigResponse{
		TxHash: res.TxHash,
	}, nil
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

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		app.eventWg.Add(1)
		go app.eventLoop()

		app.sentWg.Add(1)
		go app.handleSentToBabylonLoop()

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

		app.logger.Debug("ValidatorApp successfully stopped")

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
