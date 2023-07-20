package service

import (
	"fmt"
	"sync"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
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

	createValidatorRequestChan chan *createValidatorRequest
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
	bc bbncli.BabylonClient,
) (*ValidatorApp, error) {

	kr, err := CreateKeyring(config.KeyringDir, config.BabylonConfig.ChainID, config.KeyringBackend)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	valStore, err := val.NewValidatorStore(config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	if config.JuryMode {
		_, err := kr.Key(config.JuryKeyName)
		if err != nil {
			return nil, fmt.Errorf("the program is running in Jury mode but the Jury key %s is not found: %w",
				config.JuryKeyName, err)
		}
	}

	return &ValidatorApp{
		bc:                         bc,
		vs:                         valStore,
		kr:                         kr,
		config:                     config,
		logger:                     logger,
		quit:                       make(chan struct{}),
		createValidatorRequestChan: make(chan *createValidatorRequest),
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
	var juryPrivKey *btcec.PrivateKey
	k, err := app.kr.Key(app.config.JuryKeyName)
	if err != nil {
		return nil, err
	}
	privKey := k.GetLocal().PrivKey.GetCachedValue()
	switch v := privKey.(type) {
	case *secp256k1.PrivKey:
		juryPrivKey, _ = btcec.PrivKeyFromBytes(v.Key)
	default:
		return nil, fmt.Errorf("unsupported key type in keyring")
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
	btcPk := new(types.BIP340PubKey)
	err = btcPk.Unmarshal(validator.BtcPk)
	if err != nil {
		return nil, err
	}
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

func (app *ValidatorApp) GetValidator(pkBytes []byte) (*proto.Validator, error) {
	return app.vs.GetValidator(pkBytes)
}

func (app *ValidatorApp) handleCreateValidatorRequest(req *createValidatorRequest) (*createValidatorResponse, error) {

	kr, err := val.NewKeyringControllerWithKeyring(app.kr, req.keyName)

	if err != nil {
		return nil, fmt.Errorf("failed to create keyring controller: %w", err)
	}

	if kr.ValidatorKeyNameTaken() {
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

	btcPubKey, err := schnorr.ParsePubKey(validator.BtcPk)

	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("failed to parse created btc public key")
	}

	babylonPubKey := secp256k1.PubKey{
		Key: validator.BabylonPk,
	}

	return &createValidatorResponse{
		BtcValidatorPk:     *btcPubKey,
		BabylonValidatorPk: babylonPubKey,
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

			app.logger.WithFields(logrus.Fields{
				"btc_pub_key":     resp.BtcValidatorPk,
				"babylon_pub_key": resp.BabylonValidatorPk,
			}).Info("Successfully created validator")

			req.successResponse <- resp
		case <-app.quit:
			return
		}
	}
}
