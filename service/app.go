package service

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/valcfg"

	"github.com/babylonchain/btc-validator/val"
)

type ValidatorApp struct {
	startOnce sync.Once
	stopOnce  sync.Once

	// wg and quit are responsible for submissions go routines
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

	createValidatorRequestChan   chan *createValidatorRequest
	registerValidatorRequestChan chan *registerValidatorRequest
	addJurySigRequestChan        chan *addJurySigRequest

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
		sentQuit:                     make(chan struct{}),
		eventQuit:                    make(chan struct{}),
		createValidatorRequestChan:   make(chan *createValidatorRequest),
		registerValidatorRequestChan: make(chan *registerValidatorRequest),
		addJurySigRequestChan:        make(chan *addJurySigRequest),
		pubRandCommittedEventChan:    make(chan *pubRandCommittedEvent),
		validatorRegisteredEventChan: make(chan *validatorRegisteredEvent),
		jurySigAddedEventChan:        make(chan *jurySigAddedEvent),
		finalitySigAddedEventChan:    make(chan *finalitySigAddedEvent),
	}, nil
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
		errResponse:     make(chan error, 1),
		successResponse: make(chan *registerValidatorResponse, 1),
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
	validators, err := app.vs.ListRegisteredValidators()
	if err != nil {
		return nil, err
	}

	var finalitySigRequests []*addFinalitySigRequest
	// only submit finality signature if the validator has power at the current block height
	// 1. Fist build all the requests, based on local and babylon data
	for _, v := range validators {
		btcPk := v.MustGetBIP340BTCPK()
		power, err := app.bc.QueryValidatorVotingPower(btcPk, b.Height)
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
				if err := app.vs.SetValidatorStatus(v, proto.ValidatorStatus_INACTIVE); err != nil {
					return nil, fmt.Errorf("cannot save the validator object %s into DB: %w", v.GetBabylonPkHexString(), err)
				}
			}
			app.logger.WithFields(logrus.Fields{
				"val_btc_pk": btcPk.MarshalHex(),
				"bbn_height": b.Height,
			}).Debug("the validator's voting power is 0, skip voting")
			continue
		}

		if v.Status == proto.ValidatorStatus_REGISTERED || v.Status == proto.ValidatorStatus_INACTIVE {
			if err := app.vs.SetValidatorStatus(v, proto.ValidatorStatus_ACTIVE); err != nil {
				return nil, fmt.Errorf("cannot save the validator object %s into DB: %w", v.GetBabylonPkHexString(), err)
			}
		}
		if v.LastVotedHeight >= b.Height {
			app.logger.WithFields(logrus.Fields{
				"err":               err,
				"val_btc_pk":        btcPk.MarshalHex(),
				"bbn_height":        b.Height,
				"last_voted_height": v.LastVotedHeight,
			}).Debug("the validator's last voted height should be less than the current block height")
			continue
		}

		// build proper finality signature request
		request, err := app.buildFinalitySigRequest(v, b)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"err":               err,
				"val_btc_pk":        btcPk.MarshalHex(),
				"bbn_height":        b.Height,
				"last_voted_height": v.LastVotedHeight,
			}).Debug("failed to build finality signature request")
			continue
		}

		finalitySigRequests = append(finalitySigRequests, request)
	}

	// 2. Then submit all the requests
	var eg errgroup.Group
	var responses []*addFinalitySigResponse
	var mu sync.Mutex

	for _, request := range finalitySigRequests {
		req := request
		eg.Go(func() error {
			txHash, _, err := app.bc.SubmitFinalitySig(req.valBtcPk, req.blockHeight, req.blockLastCommitHash, req.sig)
			mu.Lock()
			defer mu.Unlock()

			// Do not return errors, as errgroup cancels other requests in case of errors.
			if err != nil {
				responses = append(responses, &addFinalitySigResponse{
					txHash:    nil,
					err:       err,
					bbnPubKey: req.bbnPubKey,
				})
			} else {
				responses = append(responses, &addFinalitySigResponse{
					txHash:    txHash,
					height:    req.blockHeight,
					err:       nil,
					bbnPubKey: req.bbnPubKey,
				})
			}
			return nil
		})
	}

	// should not happen as we do not return errors from our go routines
	if err := eg.Wait(); err != nil {
		app.logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("Unexpected error when waiting for finality signature submissions")
	}

	// 3. Check which requests succeed and bump the LastVotedHeight for each succeed one
	// report errors for failed ones.
	for _, response := range responses {
		res := response

		if res.err != nil {
			// we got the error, log it and continue.
			// TODO. this is of course not correct. check issue: https://github.com/babylonchain/btc-validator/issues/34
			app.logger.WithFields(logrus.Fields{
				"err":        res.err,
				"bbn_pk":     res.bbnPubKey,
				"bbn_height": res.height,
			}).Error("failed to submit finality signature")
			continue
		}

		respChannel := make(chan struct{}, 1)

		app.finalitySigAddedEventChan <- &finalitySigAddedEvent{
			bbnPubKey: res.bbnPubKey,
			height:    res.height,
			txHash:    res.txHash,
			// pass the channel to the event so that we can send the response to the user which requested
			// the registration
			successResponse: respChannel,
		}

		select {
		case <-respChannel:
			app.logger.WithFields(logrus.Fields{
				"bbn_height": res.height,
				"bbn_pk":     res.bbnPubKey,
			}).Debug("successfully updated last voted height in db")
		case <-app.quit:
			return nil, fmt.Errorf("validator app is shutting down")
		}

		txHashes = append(txHashes, res.txHash)
	}

	return txHashes, nil
}

func (app *ValidatorApp) buildFinalitySigRequest(v *proto.Validator, b *BlockInfo) (*addFinalitySigRequest, error) {
	privRand, err := app.GetCommittedPrivPubRand(v.BabylonPk, b.Height)
	if err != nil {
		return nil, err
	}

	btcPrivKey, err := app.getBtcPrivKey(v.KeyName)
	if err != nil {
		return nil, err
	}

	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            v.MustGetBIP340BTCPK(),
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(btcPrivKey, privRand, msgToSign)
	if err != nil {
		return nil, err
	}
	eotsSig := types.NewSchnorrEOTSSigFromModNScalar(sig)

	return &addFinalitySigRequest{
		bbnPubKey:           v.GetBabylonPK(),
		valBtcPk:            v.MustGetBIP340BTCPK(),
		blockHeight:         b.Height,
		blockLastCommitHash: b.LastCommitHash,
		sig:                 eotsSig,
	}, nil
}

// SubmitFinalitySignatureForValidator submits a finality signature for a given validator
// NOTE: this function is only called for testing double-signing so we don't want it to change
// the status of the validator
func (app *ValidatorApp) SubmitFinalitySignatureForValidator(b *BlockInfo, validator *proto.Validator) ([]byte, *btcec.PrivateKey, error) {
	req, err := app.buildFinalitySigRequest(validator, b)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build finality sig request: %w", err)
	}

	return app.bc.SubmitFinalitySig(req.valBtcPk, req.blockHeight, req.blockLastCommitHash, req.sig)
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
		stakingTxHash:   stakingMsgTx.TxHash().String(),
		sig:             jurySig,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *addJurySigResponse, 1),
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
func (app *ValidatorApp) CommitPubRandForAll(latestBbnBlock *BlockInfo) ([][]byte, error) {
	var txHashes [][]byte
	// get all the registered validators from the local storage
	validators, err := app.vs.ListRegisteredValidators()
	if err != nil {
		return nil, err
	}

	var commitPubRandRequests []*commitPubRandRequest
	// 1. Fist build all the requests, based on local and babylon data
	for _, v := range validators {
		bip340BTCPK := v.MustGetBIP340BTCPK()
		lastCommittedHeight, err := app.bc.QueryHeightWithLastPubRand(bip340BTCPK)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"err":                  err,
				"current_block_height": latestBbnBlock.Height,
				"btc_pk_hex":           bip340BTCPK.MustToBTCPK(),
			}).Debug("failed to query last committed height")
			continue
		}

		if v.LastCommittedHeight != lastCommittedHeight {
			// for some reason number of random numbers locally does not match babylon node
			// log it and try to recover somehow
			app.logger.WithFields(logrus.Fields{
				"err":                          err,
				"current_block_height":         latestBbnBlock.Height,
				"remote_last_committed_height": lastCommittedHeight,
				"local_last_committed_height":  v.LastCommittedHeight,
				"btc_pk_hex":                   bip340BTCPK.MustToBTCPK(),
			}).Debug("the validator's local last committed height does not match the remote last committed height")
			continue
		}

		var startHeight uint64
		if lastCommittedHeight == uint64(0) {
			// the validator has never submitted public rand before
			startHeight = latestBbnBlock.Height + 1
		} else if lastCommittedHeight-latestBbnBlock.Height < app.config.MinRandHeightGap {
			// we are running out of the randomness
			startHeight = lastCommittedHeight + 1
		} else {
			app.logger.WithFields(logrus.Fields{
				"last_committed_height": lastCommittedHeight,
				"current_block_height":  latestBbnBlock.Height,
				"btc_pk_hex":            bip340BTCPK.MustToBTCPK(),
			}).Debug("the validator has sufficient public randomness, skip committing more")
			continue
		}

		// generate a list of Schnorr randomness pairs
		privRandList, pubRandList, err := GenerateRandPairList(app.config.NumPubRand)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"btc_pk_hex": bip340BTCPK.MustToBTCPK(),
			}).Debug("failed to generate randomness")
			continue
		}

		// get the message hash for signing
		msg := &ftypes.MsgCommitPubRandList{
			ValBtcPk:    bip340BTCPK,
			StartHeight: startHeight,
			PubRandList: pubRandList,
		}
		hash, err := msg.HashToSign()
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"btc_pk_hex": bip340BTCPK.MustToBTCPK(),
			}).Debug("failed to sign the commit randomness message")
			continue
		}

		// sign the message hash using the validator's BTC private key
		kc, err := val.NewKeyringControllerWithKeyring(app.kr, v.KeyName)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"btc_pk_hex": bip340BTCPK.MustToBTCPK(),
			}).Debug("failed to get keyring")
			continue
		}
		schnorrSig, err := kc.SchnorrSign(hash)
		if err != nil {
			app.logger.WithFields(logrus.Fields{
				"btc_pk_hex": bip340BTCPK.MustToBTCPK(),
			}).Debug("failed to sign Schnorr signature")
			continue
		}
		sig := types.NewBIP340SignatureFromBTCSig(schnorrSig)

		request := &commitPubRandRequest{
			startingHeight: startHeight,
			bbnPubKey:      v.GetBabylonPK(),
			valBtcPk:       v.MustGetBIP340BTCPK(),
			privRandList:   privRandList,
			pubRandList:    pubRandList,
			sig:            &sig,
		}

		commitPubRandRequests = append(commitPubRandRequests, request)
	}

	// 2. Then submit all the requests
	var eg errgroup.Group
	var responses []*commitPubRandResponse
	var mu sync.Mutex

	for _, request := range commitPubRandRequests {
		req := request
		eg.Go(func() error {
			txHash, err := app.bc.CommitPubRandList(req.valBtcPk, req.startingHeight, req.pubRandList, req.sig)
			mu.Lock()
			defer mu.Unlock()

			// Do not return errors, as errgroup cancels other requests in case of errors.
			if err != nil {
				responses = append(responses, &commitPubRandResponse{
					txHash:    nil,
					err:       err,
					bbnPubKey: req.bbnPubKey,
				})
			} else {
				responses = append(responses, &commitPubRandResponse{
					txHash:       txHash,
					startHeight:  req.startingHeight,
					privRandList: req.privRandList,
					pubRandList:  req.pubRandList,
					err:          nil,
					bbnPubKey:    req.bbnPubKey,
				})
			}
			return nil
		})
	}

	// should not happen as we do not return errors from our go routines
	if err := eg.Wait(); err != nil {
		app.logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("Unexpected error when waiting for public randomness commitment")
	}

	// 3. Check which requests succeed and bump the LastCommittedHeight for each succeed one
	// report errors for failed ones.
	for _, response := range responses {
		res := response

		if res.err != nil {
			// we got the error, log it and continue.
			// TODO. this is of course not correct. check issue: https://github.com/babylonchain/btc-validator/issues/34
			app.logger.WithFields(logrus.Fields{
				"err":          res.err,
				"bbn_pk":       res.bbnPubKey,
				"start_height": res.startHeight,
			}).Error("failed to commit public randomness")
			continue
		}

		respChannel := make(chan struct{}, 1)

		app.pubRandCommittedEventChan <- &pubRandCommittedEvent{
			bbnPubKey:      res.bbnPubKey,
			startingHeight: res.startHeight,
			privRandList:   res.privRandList,
			pubRandList:    res.pubRandList,
			txHash:         res.txHash,
			// pass the channel to the event so that we can send the response to the user which requested
			// the registration
			successResponse: respChannel,
		}

		select {
		case <-respChannel:
			app.logger.WithFields(logrus.Fields{
				"start_height":   res.startHeight,
				"bbn_pk":         res.bbnPubKey,
				"num_randomness": len(res.pubRandList),
			}).Debug("successfully updated public randomness list in db")
		case <-app.quit:
			return nil, fmt.Errorf("validator app is shutting down")
		}

		txHashes = append(txHashes, res.txHash)
	}

	return txHashes, nil
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

		err = app.poller.Start(startHeight)
		if err != nil {
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

		// Always stop the submission loop first to not generate addional events and actions
		app.logger.Debug("Stopping submission loop")
		close(app.quit)
		app.wg.Wait()

		app.logger.Debug("Sent to Babylon loop stopped")
		close(app.sentQuit)
		app.sentWg.Wait()

		app.logger.Debug("Stopping main eventLoop")
		close(app.eventQuit)
		app.eventWg.Wait()

		// Closing db as last to avoid anybody to write do db
		app.logger.Debug("Stopping data store")
		err = app.vs.Close()
		if err != nil {
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

func (app *ValidatorApp) GetCommittedPrivPubRand(pkBytes []byte, height uint64) (*eots.PrivateRand, error) {
	randPair, err := app.vs.GetRandPair(pkBytes, height)
	if err != nil {
		return nil, err
	}

	if len(randPair.SecRand) != 32 {
		return nil, fmt.Errorf("the private randomness should be 32 bytes")
	}

	privRand := new(eots.PrivateRand)
	privRand.SetByteSlice(randPair.SecRand)

	return privRand, nil
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
