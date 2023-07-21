package service

import (
	"fmt"
	"sync"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
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
	poller *ChainPoller

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	commitPubRandRequestChan     chan *commitPubRandRequest

	validatorRegisteredEventChan chan *validatorRegisteredEvent
	pubRandCommittedEventChan    chan *pubRandCommittedEvent
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
		quit:                         make(chan struct{}),
		createValidatorRequestChan:   make(chan *createValidatorRequest),
		registerValidatorRequestChan: make(chan *registerValidatorRequest),
		commitPubRandRequestChan:     make(chan *commitPubRandRequest),
		pubRandCommittedEventChan:    make(chan *pubRandCommittedEvent),
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

type commitPubRandRequest struct {
	startingHeight uint64
	bbnPubKey      *secp256k1.PubKey
	valBtcPk       *types.BIP340PubKey
	privRandList   []*eots.PrivateRand
	pubRandList    []types.SchnorrPubRand
	sig            *types.BIP340Signature

	errResponse     chan error
	successResponse chan *commitPubRandResponse
}

type commitPubRandResponse struct {
	txHash []byte
}

type pubRandCommittedEvent struct {
	startingHeight  uint64
	bbnPubKey       *secp256k1.PubKey
	valBtcPk        *types.BIP340PubKey
	pubRandList     []types.SchnorrPubRand
	privRandList    []*eots.PrivateRand
	txHash          []byte
	successResponse chan *commitPubRandResponse
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

func (app *ValidatorApp) RegisterValidator(keyName string) ([]byte, error) {
	kc, err := val.NewKeyringControllerWithKeyring(app.kr, keyName)
	if err != nil {
		return nil, err
	}
	if !kc.ValidatorKeyExists() {
		return nil, fmt.Errorf("key name %s does not exist", keyName)
	}
	babylonPublicKeyBytes, err := kc.GetBabylonPublicKeyBytes()
	if err != nil {
		return nil, err
	}
	validator, err := app.vs.GetValidator(babylonPublicKeyBytes)
	if err != nil {
		return nil, err
	}

	if validator.Status != proto.ValidatorStatus_VALIDATOR_STATUS_CREATED {
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

func (app *ValidatorApp) AddJurySignature(btcDel *bstypes.BTCDelegation) ([]byte, error) {
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
		&chaincfg.SimNetParams,
	)
	if err != nil {
		return nil, err
	}

	return app.bc.SubmitJurySig(btcDel.ValBtcPk, btcDel.BtcPk, jurySig)
}

func (app *ValidatorApp) getJuryPrivKey() (*btcec.PrivateKey, error) {
	var juryPrivKey *btcec.PrivateKey
	k, err := app.kr.Key(app.config.JuryModeConfig.JuryKeyName)
	if err != nil {
		return nil, err
	}
	privKey := k.GetLocal().PrivKey.GetCachedValue()
	switch v := privKey.(type) {
	case *secp256k1.PrivKey:
		juryPrivKey, _ = btcec.PrivKeyFromBytes(v.Key)
		return juryPrivKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
	}
}

// CommitPubRandForAll generates a list of Schnorr rand pairs,
// commits the public randomness for the managed validators,
// and save the randomness pair to DB
func (app *ValidatorApp) CommitPubRandForAll(b *BlockInfo) ([][]byte, error) {
	var txHashes [][]byte
	validators, err := app.vs.ListValidators()
	if err != nil {
		return nil, err
	}

	for _, v := range validators {
		txHash, err := app.CommitPubRandForValidator(b, v)
		if err != nil {
			return nil, err
		}
		if txHash != nil {
			txHashes = append(txHashes, txHash)
		} else {
			app.logger.WithFields(logrus.Fields{
				"btc_pub_key":           v.MustGetBIP340BTCPK(),
				"block_height":          b.Height,
				"last_committed_height": v.LastCommittedHeight,
			}).Debug("the validator has sufficient committed randomness")
		}
	}

	return txHashes, nil
}

// CommitPubRandForValidator asks Babylon whether the given
// validator's public randomness has run out
// if so, generates commit public randomness request
func (app *ValidatorApp) CommitPubRandForValidator(b *BlockInfo, validator *proto.Validator) ([]byte, error) {
	pkStr := validator.MustGetBtcPubKeyHexStr()
	h, err := app.bc.QueryHeightWithLastPubRand(pkStr)
	if err != nil {
		return nil, err
	}

	if validator.LastCommittedHeight != h {
		// for some reason number of random numbers locally does not match babylon node
		// log it and try to recover somehow
		return nil, fmt.Errorf("the local last committed height %v does not match the remote last committed height %v",
			validator.LastCommittedHeight, h)
	}

	var startHeight uint64
	// the validator has never submitted public rand before
	if h == uint64(0) {
		startHeight = b.Height + 1
	} else if h-b.Height < app.config.MinRandomGap {
		startHeight = h + 1
	} else {
		return nil, nil
	}

	// generate a list of Schnorr randomness pairs
	privRandList, pubRandList, err := GenerateRandPairList(app.config.RandomNum)
	if err != nil {
		return nil, err
	}

	// get the message hash for signing
	btcPk := validator.MustGetBIP340BTCPK()
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

	request := &commitPubRandRequest{
		startingHeight:  startHeight,
		bbnPubKey:       validator.GetBabylonPK(),
		valBtcPk:        validator.MustGetBIP340BTCPK(),
		privRandList:    privRandList,
		pubRandList:     pubRandList,
		errResponse:     make(chan error),
		successResponse: make(chan *commitPubRandResponse),
		sig:             &sig,
	}

	app.commitPubRandRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse.txHash, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

func (app *ValidatorApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Infof("Starting ValidatorApp")

		app.wg.Add(3)
		go app.handleSentToBabylonLoop()
		go app.eventLoop()
		go app.newBabylonBlockLoop()
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
	app.logger.WithFields(logrus.Fields{ // TODO: use hex format
		"btc_pub_key":     btcPubKey,
		"babylon_pub_key": babylonPubKey,
	}).Debug("created validator")

	return &createValidatorResponse{
		BtcValidatorPk:     *btcPubKey,
		BabylonValidatorPk: *babylonPubKey,
	}, nil
}

func (app *ValidatorApp) newBabylonBlockLoop() {
	defer app.wg.Done()

	for {
		select {
		case b := <-app.poller.GetBlockInfoChan():
			// TODO: add a new trigger for CommitPubRand as doing this upon every
			// block is kinda overkill
			_, err := app.CommitPubRandForAll(b)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"block_height": b.Height,
					"err":          err,
				}).Error("failed to commit public randomness")
				continue
			}

		// TODO ask Babylon whether there are any delegations need jury sig if the program is running in Jury mode

		// TODO ask Babylon whether finality vote is needed
		case <-app.quit:
			return
		}
	}
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
				}).Fatal("err while saving validator to DB")
			}

			// return to the caller
			ev.successResponse <- &registerValidatorResponse{
				txHash: ev.txHash,
			}

		case ev := <-app.pubRandCommittedEventChan:
			val, err := app.vs.GetValidator(ev.bbnPubKey.Key)
			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("Public randomness committed validator not found in DB")
			}

			val.LastCommittedHeight = ev.startingHeight + uint64(len(ev.pubRandList)-1)

			// save the updated validator object to DB
			err = app.vs.SaveValidator(val)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("err while saving validator to DB")
			}

			// save the committed random list to DB
			// TODO 1: Optimize the db interface to batch the saving operations
			// TODO 2: Consider safety after recovery
			for i, pr := range ev.privRandList {
				height := ev.startingHeight + uint64(i)
				privRand := pr.Bytes()
				randPair := &proto.SchnorrRandPair{
					SecRand: privRand[:],
					PubRand: ev.pubRandList[i].MustMarshal(),
				}
				err = app.vs.SaveRandPair(ev.bbnPubKey.Key, height, randPair)
				if err != nil {
					app.logger.WithFields(logrus.Fields{
						"bbn_pk": ev.bbnPubKey,
					}).Fatal("err while saving committed random pair to DB")
				}
			}

			// return to the caller
			ev.successResponse <- &commitPubRandResponse{
				txHash: ev.txHash,
			}

		case <-app.quit:
			return
		}
	}
}

// Loop for handling requests to send stuff to babylon. It is necessart to properly
// serialize bayblon sends as otherwise we would keep hitting sequence mismatch errors.
// This could be done either by send loop or by lock. We choose send loop as it is
// more flexible.
// TODO: This could be probably separate component responsible for queuing stuff
// and sending it to babylon.
func (app *ValidatorApp) handleSentToBabylonLoop() {
	defer app.wg.Done()
	for {
		select {
		case req := <-app.registerValidatorRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it fails due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			tx, err := app.bc.RegisterValidator(req.bbnPubKey, req.btcPubKey, req.pop)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":       err,
					"bbnPubKey": req.bbnPubKey,
					"btcPubKey": req.btcPubKey,
				}).Error("failed to register validator")
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
		case req := <-app.commitPubRandRequestChan:
			tx, err := app.bc.CommitPubRandList(req.valBtcPk, req.startingHeight, req.pubRandList, req.sig)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":         err,
					"btcPubKey":   req.valBtcPk,
					"startHeight": req.startingHeight,
				})
				req.errResponse <- err
				continue
			}

			app.logger.WithField("btcPk", req.valBtcPk).Info("successfully commit public rand list on babylon")

			app.pubRandCommittedEventChan <- &pubRandCommittedEvent{
				startingHeight: req.startingHeight,
				bbnPubKey:      req.bbnPubKey,
				valBtcPk:       req.valBtcPk,
				privRandList:   req.privRandList,
				pubRandList:    req.pubRandList,
				txHash:         tx,
				// pass the channel to the event so that we can send the response to the user which requested
				// the commit
				successResponse: req.successResponse,
			}

		case <-app.quit:
			return
		}
	}
}
