package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"

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
}

func NewValidatorAppFromConfig(
	config *valcfg.Config,
	logger *logrus.Logger,
	bc bbncli.BabylonClient,
) (*ValidatorApp, error) {

	kr, err := CreateKeyring(config.KeyringDir, config.ChainID, config.KeyringBackend)
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
		bc:     bc,
		vs:     valStore,
		kr:     kr,
		config: config,
		logger: logger,
		quit:   make(chan struct{}),
	}, nil
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
	// TODO: Optimize the db interface to batch the saving operations
	for i := 0; i < int(num); i++ {
		height := startHeight + uint64(i)
		privRand := privRandList[i].Bytes()
		randPair := &valrpc.SchnorrRandPair{
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

// main event loop for the validator app
func (app *ValidatorApp) eventLoop() {
	panic("implement me")
}

func GenerateRandPairList(num uint64) ([]*eots.PrivateRand, []types.SchnorrPubRand, error) {
	srList := make([]*eots.PrivateRand, num)
	prList := make([]types.SchnorrPubRand, num)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := uint64(0); i < num; i++ {
		eotsSR, eotsPR, err := eots.RandGen(r)
		if err != nil {
			return nil, nil, err
		}
		pr := types.NewSchnorrPubRandFromFieldVal(eotsPR)
		srList[i] = eotsSR
		prList[i] = *pr
	}
	return srList, prList, nil
}
