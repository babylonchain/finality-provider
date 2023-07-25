package service

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
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
	poller *ChainPoller

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	commitPubRandRequestChan     chan *commitPubRandRequest
	addJurySigRequestChan        chan *addJurySigRequest
	addFinalitySigRequestChan    chan *addFinalitySigRequest

	validatorRegisteredEventChan chan *validatorRegisteredEvent
	pubRandCommittedEventChan    chan *pubRandCommittedEvent
	jurySigAddedEventChan        chan *jurySigAddedEvent
	finalitySigAddedEventChan    chan *finalitySigAddedEvent
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
		addJurySigRequestChan:        make(chan *addJurySigRequest),
		addFinalitySigRequestChan:    make(chan *addFinalitySigRequest),
		pubRandCommittedEventChan:    make(chan *pubRandCommittedEvent),
		validatorRegisteredEventChan: make(chan *validatorRegisteredEvent),
		jurySigAddedEventChan:        make(chan *jurySigAddedEvent),
		finalitySigAddedEventChan:    make(chan *finalitySigAddedEvent),
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

type addJurySigRequest struct {
	bbnPubKey       *secp256k1.PubKey
	valBtcPk        *types.BIP340PubKey
	delBtcPk        *types.BIP340PubKey
	sig             *types.BIP340Signature
	errResponse     chan error
	successResponse chan *addJurySigResponse
}

type addJurySigResponse struct {
	txHash []byte
}

type jurySigAddedEvent struct {
	bbnPubKey       *secp256k1.PubKey
	txHash          []byte
	successResponse chan *addJurySigResponse
}

type addFinalitySigRequest struct {
	bbnPubKey           *secp256k1.PubKey
	valBtcPk            *types.BIP340PubKey
	blockHeight         uint64
	blockLastCommitHash []byte
	sig                 *types.SchnorrEOTSSig
	errResponse         chan error
	successResponse     chan *addFinalitySigResponse
}

type addFinalitySigResponse struct {
	txHash []byte
}

type finalitySigAddedEvent struct {
	bbnPubKey       *secp256k1.PubKey
	height          uint64
	txHash          []byte
	successResponse chan *addFinalitySigResponse
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

	if validator.Status != proto.ValidatorStatus_CREATED {
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

// SubmitFinalitySignaturesForAll signs and submits finality signatures to Babylon
// for all the managed validators at the given Babylon block height
func (app *ValidatorApp) SubmitFinalitySignaturesForAll(b *BlockInfo) ([][]byte, error) {
	// get all the managed validators
	var txHashes [][]byte
	validators, err := app.vs.ListValidators()
	if err != nil {
		return nil, err
	}

	// only submit finality signature if the validator has power at the current block height
	for _, v := range validators {
		btcPk := v.MustGetBIP340BTCPK()
		power, err := app.bc.QueryValidatorVotePower(btcPk, b.Height)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"err":        err,
				"val_btc_pk": btcPk.MarshalHex(),
				"bbn_height": b.Height,
			}).Error("failed to check whether the validator should vote")
			continue
		}
		if power == 0 {
			if v.Status == proto.ValidatorStatus_ACTIVE {
				v.Status = proto.ValidatorStatus_INACTIVE
				if err := app.vs.SaveValidator(v); err != nil {
					return nil, fmt.Errorf("cannot save the validator object %s into DB: %w", v.GetBabylonPkHexString(), err)
				}
			}
			continue
		}
		if v.LastVotedHeight >= b.Height {
			app.logger.WithFields(logrus.Fields{
				"err":               err,
				"val_btc_pk":        btcPk.MarshalHex(),
				"bbn_height":        b.Height,
				"last_voted_height": v.LastVotedHeight,
			}).Error("the validator's last voted height should less than the current block height")
			continue
		}
		txHash, err := app.submitFinalitySignatureForValidator(b, v)
		if err != nil {
			return nil, fmt.Errorf("failed to submit the finality signature from validator %s to Babylon: %w",
				v.GetBabylonPkHexString(), err)
		}
		txHashes = append(txHashes, txHash)
	}

	return txHashes, nil
}

func (app *ValidatorApp) submitFinalitySignatureForValidator(b *BlockInfo, validator *proto.Validator) ([]byte, error) {
	pair, err := app.GetCommittedPubRandPair(validator.BabylonPk, b.Height)
	if err != nil {
		return nil, err
	}

	privRand := new(eots.PrivateRand)
	privRand.SetByteSlice(pair.SecRand)

	btcPrivKey, err := app.getBtcPrivKey(validator.KeyName)
	if err != nil {
		return nil, err
	}

	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            validator.MustGetBIP340BTCPK(),
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(btcPrivKey, privRand, msgToSign)
	if err != nil {
		return nil, err
	}
	eotsSig := types.NewSchnorrEOTSSigFromModNScalar(sig)

	request := &addFinalitySigRequest{
		bbnPubKey:           validator.GetBabylonPK(),
		valBtcPk:            validator.MustGetBIP340BTCPK(),
		blockHeight:         b.Height,
		blockLastCommitHash: b.LastCommitHash,
		sig:                 eotsSig,
		errResponse:         make(chan error),
		successResponse:     make(chan *addFinalitySigResponse),
	}

	app.addFinalitySigRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse.txHash, nil
	case <-app.quit:
		return nil, fmt.Errorf("validator app is shutting down")
	}
}

// AddJurySignature adds a Jury signature on the given Bitcoin delegation and submits it to Babylon
// Note: this should be only called when the program is running in Jury mode
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
		&app.config.JuryModeConfig.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	request := &addJurySigRequest{
		bbnPubKey:       btcDel.BabylonPk,
		valBtcPk:        btcDel.ValBtcPk,
		delBtcPk:        btcDel.BtcPk,
		sig:             jurySig,
		errResponse:     make(chan error),
		successResponse: make(chan *addJurySigResponse),
	}

	app.addJurySigRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse.txHash, nil
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
		// skip validators whose status is still CREATED
		if v.Status == proto.ValidatorStatus_CREATED {
			continue
		}
		txHash, err := app.commitPubRandForValidator(b, v)
		if err != nil {
			return nil, err
		}
		if txHash != nil {
			txHashes = append(txHashes, txHash)
		} else {
			app.logger.WithFields(logrus.Fields{
				"btc_pub_key":           v.MustGetBIP340BTCPK().MarshalHex(),
				"block_height":          b.Height,
				"last_committed_height": v.LastCommittedHeight,
			}).Debug("the validator has sufficient committed randomness")
		}
	}

	return txHashes, nil
}

// commitPubRandForValidator asks Babylon whether the given
// validator's public randomness has run out
// if so, generates commit public randomness request
func (app *ValidatorApp) commitPubRandForValidator(latestBbnBlock *BlockInfo, validator *proto.Validator) ([]byte, error) {
	bip340BTCPK := validator.MustGetBIP340BTCPK()
	lastCommittedHeight, err := app.bc.QueryHeightWithLastPubRand(bip340BTCPK)
	if err != nil {
		return nil, err
	}

	if validator.LastCommittedHeight != lastCommittedHeight {
		// for some reason number of random numbers locally does not match babylon node
		// log it and try to recover somehow
		return nil, fmt.Errorf("the local last committed height %v does not match the remote last committed height %v",
			validator.LastCommittedHeight, lastCommittedHeight)
	}

	var startHeight uint64
	// the validator has never submitted public rand before
	if lastCommittedHeight == uint64(0) {
		startHeight = latestBbnBlock.Height + 1
	} else if lastCommittedHeight-latestBbnBlock.Height < app.config.MinRandHeightGap {
		startHeight = lastCommittedHeight + 1
	} else {
		return nil, nil
	}

	// generate a list of Schnorr randomness pairs
	privRandList, pubRandList, err := GenerateRandPairList(app.config.NumPubRand)
	if err != nil {
		return nil, err
	}

	// get the message hash for signing
	msg := &ftypes.MsgCommitPubRandList{
		ValBtcPk:    bip340BTCPK,
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

		err := app.poller.Start()
		if err != nil {
			startErr = err
			return
		}

		app.wg.Add(3)
		go app.handleSentToBabylonLoop()
		go app.eventLoop()
		if !app.IsJury() {
			go app.validatorSubmissionLoop()
		} else {
			go app.jurySigSubmissionLoop()
		}
	})

	return startErr
}

func (app *ValidatorApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Infof("Stopping ValidatorApp")
		err := app.poller.Stop()
		if err != nil {
			stopErr = err
			return
		}
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

func (app *ValidatorApp) IsJury() bool {
	return app.config.JuryMode
}

func (app *ValidatorApp) ListValidators() ([]*proto.Validator, error) {
	return app.vs.ListValidators()
}

func (app *ValidatorApp) GetValidator(pkBytes []byte) (*proto.Validator, error) {
	return app.vs.GetValidator(pkBytes)
}

// GetCommittedPubRandPairList gets all the public randomness pairs from DB with the descending order
func (app *ValidatorApp) GetCommittedPubRandPairList(pkBytes []byte) ([]*proto.SchnorrRandPair, error) {
	return app.vs.GetRandPairList(pkBytes)
}

func (app *ValidatorApp) GetCommittedPubRandPair(pkBytes []byte, height uint64) (*proto.SchnorrRandPair, error) {
	return app.vs.GetRandPair(pkBytes, height)
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

func (app *ValidatorApp) getPendingDelegationsForAll() ([]*btcstakingtypes.BTCDelegation, error) {
	var delegations []*btcstakingtypes.BTCDelegation

	dels, err := app.bc.QueryPendingBTCDelegations()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending BTC delegations: %w", err)
	}
	delegations = append(delegations, dels...)

	return delegations, nil
}

// jurySigSubmissionLoop is the reactor to submit Jury signature for pending BTC delegations
func (app *ValidatorApp) jurySigSubmissionLoop() {
	defer app.wg.Done()

	interval := app.config.JuryModeConfig.QueryInterval
	jurySigTicker := time.NewTicker(interval)

	for {
		select {
		case <-jurySigTicker.C:
			dels, err := app.getPendingDelegationsForAll()
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get pending delegations")
				continue
			}

			for _, d := range dels {
				_, err := app.AddJurySignature(d)
				if err != nil {
					app.logger.WithFields(logrus.Fields{
						"err":        err,
						"del_btc_pk": d.BtcPk,
					}).Error("failed to submit Jury sig to the Bitcoin delegation")
				}
			}

		case <-app.quit:
			return
		}
	}

}

// validatorSubmissionLoop is the reactor to submit finality signature and public randomness
func (app *ValidatorApp) validatorSubmissionLoop() {
	defer app.wg.Done()

	commitRandTicker := time.NewTicker(app.config.RandomnessCommitInterval)

	for {
		select {
		case b := <-app.poller.GetBlockInfoChan():
			app.logger.WithFields(logrus.Fields{
				"bbn_height": b.Height,
			}).Debug("Submitting finality signatures for managed validators")
			_, err := app.SubmitFinalitySignaturesForAll(b)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":        err,
					"bbn_height": b.Height,
				}).Error("failed to submit finality signature to Babylon")
			}
		case <-commitRandTicker.C:
			lastBlock, err := app.GetCurrentBbnBlock()
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err": err,
				}).Fatal("failed to get the current Babylon block")
			}
			_, err = app.CommitPubRandForAll(lastBlock)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"block_height": lastBlock.Height,
					"err":          err,
				}).Error("failed to commit public randomness")
				continue
			}
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

		case ev := <-app.finalitySigAddedEventChan:
			val, err := app.vs.GetValidator(ev.bbnPubKey.Key)

			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("finality signature added validator not found in DB")
			}

			// update the last_voted_height
			val.LastVotedHeight = ev.height
			err = app.vs.SaveValidator(val)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("err while saving validator to DB")
			}

			// return to the caller
			ev.successResponse <- &addFinalitySigResponse{
				txHash: ev.txHash,
			}

		case ev := <-app.validatorRegisteredEventChan:
			val, err := app.vs.GetValidator(ev.bbnPubKey.Key)

			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("Registred validator not found in DB")
			}

			// change the status of the validator to registered
			val.Status = proto.ValidatorStatus_REGISTERED

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
		case ev := <-app.jurySigAddedEventChan:
			// TODO do we assume the delegator is also a BTC validator?
			// if so, do we want to change its status to ACTIVE here?
			// if not, maybe we can remove the handler of this event

			// return to the caller
			ev.successResponse <- &addJurySigResponse{
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
		case req := <-app.addFinalitySigRequestChan:
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			tx, err := app.bc.SubmitFinalitySig(req.valBtcPk, req.blockHeight, req.blockLastCommitHash, req.sig)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":       err,
					"btcPubKey": req.valBtcPk.MarshalHex(),
					"height":    req.blockHeight,
				}).Error("failed to submit finality signature")
				req.errResponse <- err
				continue
			}

			app.logger.WithFields(logrus.Fields{
				"btcPubKey": req.valBtcPk.MarshalHex(),
				"height":    req.blockHeight,
			}).Info("successfully submitted a finality signature to babylon")

			app.finalitySigAddedEventChan <- &finalitySigAddedEvent{
				bbnPubKey: req.bbnPubKey,
				height:    req.blockHeight,
				txHash:    tx,
				// pass the channel to the event so that we can send the response to the user which requested
				// the registration
				successResponse: req.successResponse,
			}
		case req := <-app.registerValidatorRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it fails due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			tx, err := app.bc.RegisterValidator(req.bbnPubKey, req.btcPubKey, req.pop)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":       err,
					"bbnPubKey": hex.EncodeToString(req.bbnPubKey.Key),
					"btcPubKey": req.btcPubKey.MarshalHex(),
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
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			tx, err := app.bc.CommitPubRandList(req.valBtcPk, req.startingHeight, req.pubRandList, req.sig)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":         err,
					"btcPubKey":   req.valBtcPk,
					"startHeight": req.startingHeight,
				}).Error("failed to commit public randomness")
				req.errResponse <- err
				continue
			}

			app.logger.WithField("btcPk", req.valBtcPk.MarshalHex()).Info("successfully committed public rand list on babylon")

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
		case req := <-app.addJurySigRequestChan:
			// TODO: we should add some retry mechanism or we can have a health checker to check the connection periodically
			tx, err := app.bc.SubmitJurySig(req.valBtcPk, req.delBtcPk, req.sig)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":          err,
					"valBtcPubKey": req.valBtcPk,
					"delBtcPubKey": req.delBtcPk,
				}).Error("failed to submit Jury signature")
				req.errResponse <- err
				continue
			}

			app.logger.WithField("delBtcPk", req.delBtcPk).Info("successfully submit Jury sig over Bitcoin delegation to Babylon")

			app.jurySigAddedEventChan <- &jurySigAddedEvent{
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
