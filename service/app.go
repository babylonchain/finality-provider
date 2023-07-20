package service

import (
	"fmt"
	"sync"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
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
	wg        sync.WaitGroup
	quit      chan struct{}

	bc     bbncli.BabylonClient
	kr     keyring.Keyring
	vs     *val.ValidatorStore
	config *valcfg.Config
	logger *logrus.Logger

	createValidatorRequestChan chan *createValidatorRequest

	registerValidatorRequestChan chan *registerValidatorRequest
	validatorRegisteredEventChan chan *validatorRegisteredEvent
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

	if err != nil {
		return nil, err
	}

	return &ValidatorApp{
		bc:                           bc,
		vs:                           valStore,
		kr:                           kr,
		config:                       config,
		logger:                       logger,
		quit:                         make(chan struct{}),
		createValidatorRequestChan:   make(chan *createValidatorRequest),
		registerValidatorRequestChan: make(chan *registerValidatorRequest),
		validatorRegisteredEventChan: make(chan *validatorRegisteredEvent),
	}, nil
}

type createValidatorResponse struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}
type createValidatorRequest struct {
	keyName         string
	errResponse     chan error
	successResponse chan *createValidatorResponse
}

type registerValidatorRequest struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *types.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	errResponse     chan error
	successResponse chan *registerValidatorResponse
}

type validatorRegisteredEvent struct {
	bbnPubKey       *secp256k1.PubKey
	txHash          []byte
	successResponse chan *registerValidatorResponse
}

type registerValidatorResponse struct {
	txHash []byte
}

type CreateValidatorResult struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}

func (app *ValidatorApp) GetValidatorStore() *val.ValidatorStore {
	return app.vs
}

func (app *ValidatorApp) GetKeyring() keyring.Keyring {
	return app.kr
}

func (app *ValidatorApp) RegisterValidator(pkBytes []byte) ([]byte, error) {
	validator, err := app.vs.GetValidator(pkBytes)
	if err != nil {
		return nil, err
	}

	if validator.Status > proto.ValidatorStatus_VALIDATOR_STATUS_CREATED {
		return nil, fmt.Errorf("validator is already registered")
	}

	// TODO: the following decoding is not needed if Babylon and cosmos protos are introduced

	bbnPk := validator.GetBabylonPK()
	btcPk := validator.MustGetBIP340BTCPK()
	btcSig, err := types.NewBIP340Signature(validator.Pop.BtcSig)
	if err != nil {
		return nil, err
	}

	pop := &bstypes.ProofOfPossession{
		BabylonSig: validator.Pop.BabylonSig,
		BtcSig:     btcSig,
	}

	request := &registerValidatorRequest{
		bbnPubKey:       bbnPk,
		btcPubKey:       btcPk,
		pop:             pop,
		errResponse:     make(chan error),
		successResponse: make(chan *registerValidatorResponse),
	}

	app.registerValidatorRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse.txHash, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

// CommitPubRandForAll generates a list of Schnorr rand pairs,
// commits the public randomness for the managed validators,
// and save the randomness pair to DB
// Note: if pkBytes is nil, this function works for this validator.
// Otherwise, it is for all the managed validators.
func (app *ValidatorApp) CommitPubRandForAll(num uint64) ([][]byte, error) {
	var txHashes [][]byte
	validators, err := app.vs.ListValidators()
	if err != nil {
		return nil, err
	}
	for _, v := range validators {
		txHash, err := app.CommitPubRandForValidator(v.BabylonPk, num)
		if err != nil {
			return nil, err
		}
		txHashes = append(txHashes, txHash)
	}

	return txHashes, nil
}

// CommitPubRandForValidator generates, commits and saves a list of
// Schnorr random pair for a specific managed validator
func (app *ValidatorApp) CommitPubRandForValidator(pkBytes []byte, num uint64) ([]byte, error) {
	// get the managed validator object
	validator, err := app.vs.GetValidator(pkBytes)
	if err != nil {
		return nil, err
	}

	// generate a list of Schnorr randomness pairs
	privRandList, pubRandList, err := GenerateRandPairList(num)
	if err != nil {
		return nil, err
	}

	// get the message hash for signing
	btcPk := validator.MustGetBIP340BTCPK()
	startHeight := validator.LastCommittedHeight + 1
	msg := &ftypes.MsgCommitPubRandList{
		ValBtcPk:    btcPk,
		StartHeight: startHeight,
		PubRandList: pubRandList,
	}
	hash, err := msg.HashToSign()
	if err != nil {
		return nil, err
	}

	// sign the message hash using the validator's BTC private key
	kc, err := val.NewKeyringControllerWithKeyring(app.kr, validator.KeyName)
	if err != nil {
		return nil, err
	}
	schnorrSig, err := kc.SchnorrSign(hash)
	if err != nil {
		return nil, err
	}
	sig := types.NewBIP340SignatureFromBTCSig(schnorrSig)

	// commit the rand list to Babylon
	txHash, err := app.bc.CommitPubRandList(btcPk, startHeight, pubRandList, &sig)
	if err != nil {
		return nil, err
	}

	// update and save the validator object to DB
	validator.LastCommittedHeight = validator.LastCommittedHeight + num
	err = app.vs.SaveValidator(validator)
	if err != nil {
		panic(fmt.Errorf("failed to save updated validator object: %w", err))
	}

	// save the committed random list to DB
	// TODO 1: Optimize the db interface to batch the saving operations
	// TODO 2: Consider safety after recovery
	for i := 0; i < int(num); i++ {
		height := startHeight + uint64(i)
		privRand := privRandList[i].Bytes()
		randPair := &proto.SchnorrRandPair{
			SecRand: privRand[:],
			PubRand: pubRandList[i].MustMarshal(),
		}
		err = app.vs.SaveRandPair(pkBytes, height, randPair)
		if err != nil {
			return nil, err
		}
	}

	return txHash, nil
}

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		app.wg.Add(2)
		go app.handleSentToBabylonLoop()
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

func (app *ValidatorApp) CreateValidator(keyName string) (*CreateValidatorResult, error) {
	req := &createValidatorRequest{
		keyName:         keyName,
		errResponse:     make(chan error),
		successResponse: make(chan *createValidatorResponse),
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

func (app *ValidatorApp) ListValidators() ([]*proto.Validator, error) {
	return app.vs.ListValidators()
}

func (app *ValidatorApp) GetValidator(pkBytes []byte) (*proto.Validator, error) {
	return app.vs.GetValidator(pkBytes)
}

func (app *ValidatorApp) handleCreateValidatorRequest(req *createValidatorRequest) (*createValidatorResponse, error) {

	app.logger.Debug("handling CreateValidator request")

	kr, err := val.NewKeyringControllerWithKeyring(app.kr, req.keyName)

	if err != nil {
		return nil, fmt.Errorf("failed to create keyring controller: %w", err)
	}

	if kr.KeyNameTaken() {
		return nil, fmt.Errorf("the key name %s is taken", kr.GetKeyName())
	}

	// TODO should not expose direct proto here, as this is internal db representation
	// conected to serialization
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
	app.logger.WithFields(logrus.Fields{ // TODO: use hex format
		"btc_pub_key":     btcPubKey,
		"babylon_pub_key": babylonPubKey,
	}).Debug("created validator")

	return &createValidatorResponse{
		BtcValidatorPk:     *btcPubKey,
		BabylonValidatorPk: *babylonPubKey,
	}, nil
}

// main event loop for the validator app
func (app *ValidatorApp) eventLoop() {
	defer app.wg.Done()

	for {
		select {
		case req := <-app.createValidatorRequestChan:
			resp, err := app.handleCreateValidatorRequest(req)

			if err != nil {
				req.errResponse <- err
				continue
			}

			req.successResponse <- resp

		case ev := <-app.validatorRegisteredEventChan:
			val, err := app.vs.GetValidator(ev.bbnPubKey.Key)

			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("Registred validator not found in DB")
			}

			// change the status of the validator to registered
			val.Status = proto.ValidatorStatus_VALIDATOR_STATUS_REGISTERED

			// save the updated validator object to DB
			err = app.vs.SaveValidator(val)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("erroch while saving validator to DB")
			}

			// return to the caller
			ev.successResponse <- &registerValidatorResponse{
				txHash: ev.txHash,
			}

		case <-app.quit:
			return
		}
	}
}

// Loop for handling requests to send stuff to babylon. It is necessart to properly
// serialize bayblon sends as otherwise we would keep hitting sequce mismatch errors.
// This could be done either by send loop or by lock. We choose send loop as it is
// more flexible.
// TODO: This could be probably separate component responsbile for queuing stuff
// and sending it to babylon.
func (app *ValidatorApp) handleSentToBabylonLoop() {
	defer app.wg.Done()
	for {
		select {
		case req := <-app.registerValidatorRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it failes due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			tx, err := app.bc.RegisterValidator(req.bbnPubKey, req.btcPubKey, req.pop)

			if err != nil {
				app.logger.WithField("error", err).Error("failed to register validator")
				req.errResponse <- err
				continue
			}

			app.logger.WithField("bbnPk", req.bbnPubKey).Info("successfully registered validator on babylon")

			app.validatorRegisteredEventChan <- &validatorRegisteredEvent{
				bbnPubKey: req.bbnPubKey,
				txHash:    tx,
				// pass the channel to the event so that we can send the response to the user which requested
				// the registration
				successResponse: req.successResponse,
			}
		case <-app.quit:
			return
		}
	}
}
