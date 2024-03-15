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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lightningnetwork/lnd/kvdb"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/store"
	fpkr "github.com/babylonchain/finality-provider/keyring"
	"github.com/babylonchain/finality-provider/types"
)

type FinalityProviderApp struct {
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
	fps    *store.FinalityProviderStore
	config *fpcfg.Config
	logger *zap.Logger
	input  *strings.Reader

	fpManager   *FinalityProviderManager
	eotsManager eotsmanager.EOTSManager

	createFinalityProviderRequestChan   chan *createFinalityProviderRequest
	registerFinalityProviderRequestChan chan *registerFinalityProviderRequest
	finalityProviderRegisteredEventChan chan *finalityProviderRegisteredEvent
}

func NewFinalityProviderAppFromConfig(
	config *fpcfg.Config,
	db kvdb.Backend,
	logger *zap.Logger,
) (*FinalityProviderApp, error) {
	cc, err := clientcontroller.NewClientController(config.ChainName, config.BabylonConfig, &config.BTCNetParams, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client for the consumer chain %s: %v", config.ChainName, err)
	}

	// if the EOTSManagerAddress is empty, run a local EOTS manager;
	// otherwise connect a remote one with a gRPC client
	em, err := client.NewEOTSManagerGRpcClient(config.EOTSManagerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create EOTS manager client: %w", err)
	}

	logger.Info("successfully connected to a remote EOTS manager", zap.String("address", config.EOTSManagerAddress))

	return NewFinalityProviderApp(config, cc, em, db, logger)
}

func NewFinalityProviderApp(
	config *fpcfg.Config,
	cc clientcontroller.ClientController,
	em eotsmanager.EOTSManager,
	db kvdb.Backend,
	logger *zap.Logger,
) (*FinalityProviderApp, error) {
	fpStore, err := store.NewFinalityProviderStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate finality provider store: %w", err)
	}

	input := strings.NewReader("")
	kr, err := fpkr.CreateKeyring(
		config.BabylonConfig.KeyDirectory,
		config.BabylonConfig.ChainID,
		config.BabylonConfig.KeyringBackend,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	fpm, err := NewFinalityProviderManager(fpStore, config, cc, em, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality-provider manager: %w", err)
	}

	return &FinalityProviderApp{
		cc:                                  cc,
		fps:                                 fpStore,
		kr:                                  kr,
		config:                              config,
		logger:                              logger,
		input:                               input,
		fpManager:                           fpm,
		eotsManager:                         em,
		quit:                                make(chan struct{}),
		sentQuit:                            make(chan struct{}),
		eventQuit:                           make(chan struct{}),
		createFinalityProviderRequestChan:   make(chan *createFinalityProviderRequest),
		registerFinalityProviderRequestChan: make(chan *registerFinalityProviderRequest),
		finalityProviderRegisteredEventChan: make(chan *finalityProviderRegisteredEvent),
	}, nil
}

func (app *FinalityProviderApp) GetConfig() *fpcfg.Config {
	return app.config
}

func (app *FinalityProviderApp) GetFinalityProviderStore() *store.FinalityProviderStore {
	return app.fps
}

func (app *FinalityProviderApp) GetKeyring() keyring.Keyring {
	return app.kr
}

func (app *FinalityProviderApp) GetInput() *strings.Reader {
	return app.input
}

func (app *FinalityProviderApp) ListFinalityProviderInstances() []*FinalityProviderInstance {
	return app.fpManager.ListFinalityProviderInstances()
}

func (app *FinalityProviderApp) ListAllFinalityProvidersInfo() ([]*proto.FinalityProviderInfo, error) {
	return app.fpManager.AllFinalityProviders()
}

func (app *FinalityProviderApp) GetFinalityProviderInfo(fpPk *bbntypes.BIP340PubKey) (*proto.FinalityProviderInfo, error) {
	return app.fpManager.FinalityProviderInfo(fpPk)
}

// GetFinalityProviderInstance returns the finality-provider instance with the given Babylon public key
func (app *FinalityProviderApp) GetFinalityProviderInstance(fpPk *bbntypes.BIP340PubKey) (*FinalityProviderInstance, error) {
	return app.fpManager.GetFinalityProviderInstance(fpPk)
}

func (app *FinalityProviderApp) RegisterFinalityProvider(fpPkStr string) (*RegisterFinalityProviderResponse, error) {
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(fpPkStr)
	if err != nil {
		return nil, err
	}

	fp, err := app.fps.GetFinalityProvider(fpPk.MustToBTCPK())
	if err != nil {
		return nil, err
	}

	if fp.Status != proto.FinalityProviderStatus_CREATED {
		return nil, fmt.Errorf("finality-provider is already registered")
	}

	btcSig, err := bbntypes.NewBIP340Signature(fp.Pop.BtcSig)
	if err != nil {
		return nil, err
	}

	pop := &bstypes.ProofOfPossession{
		BabylonSig: fp.Pop.ChainSig,
		BtcSig:     btcSig.MustMarshal(),
		BtcSigType: bstypes.BTCSigType_BIP340,
	}

	request := &registerFinalityProviderRequest{
		bbnPubKey:       fp.ChainPk,
		btcPubKey:       bbntypes.NewBIP340PubKeyFromBTCPK(fp.BtcPk),
		pop:             pop,
		description:     fp.Description,
		commission:      fp.Commission,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *RegisterFinalityProviderResponse, 1),
	}

	app.registerFinalityProviderRequestChan <- request

	select {
	case err := <-request.errResponse:
		return nil, err
	case successResponse := <-request.successResponse:
		return successResponse, nil
	case <-app.quit:
		return nil, fmt.Errorf("finality-provider app is shutting down")
	}
}

// StartHandlingFinalityProvider starts a finality-provider instance with the given Babylon public key
// Note: this should be called right after the finality-provider is registered
func (app *FinalityProviderApp) StartHandlingFinalityProvider(fpPk *bbntypes.BIP340PubKey, passphrase string) error {
	return app.fpManager.StartFinalityProvider(fpPk, passphrase)
}

func (app *FinalityProviderApp) StartHandlingAll() error {
	return app.fpManager.StartAll()
}

// NOTE: this is not safe in production, so only used for testing purpose
func (app *FinalityProviderApp) getFpPrivKey(fpPk []byte) (*btcec.PrivateKey, error) {
	record, err := app.eotsManager.KeyRecord(fpPk, "")
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
}

// Sync the finality-provider status
// It will update the fp status CREATED to REGISTERED in the fpd database,
// if the fp appears to be registered in the Babylon chain.
func (app *FinalityProviderApp) SyncFinalityProviderStatus() error {
	fps, err := app.fps.GetAllStoredFinalityProviders()
	if err != nil {
		return err
	}

	fmt.Println(fps)

	for _, fp := range fps {
		// QueryFinalityProviderSlashed will not occur any error,
		// if fp is registered in the Babylon chain.
		_, err = app.cc.QueryFinalityProviderSlashed(fp.BtcPk)
		if err != nil {
			// if error occured then the finality-provider is not registered in the Babylon chain
			continue
		}

		err = app.fps.SetFpStatus(fp.BtcPk, proto.FinalityProviderStatus_REGISTERED)
		if err != nil {
			return err
		}
	}

	return nil
}

// Start starts only the finality-provider daemon without any finality-provider instances
func (app *FinalityProviderApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Info("Starting FinalityProviderApp")

		app.eventWg.Add(1)
		go app.eventLoop()

		app.sentWg.Add(1)
		go app.registrationLoop()
	})

	return startErr
}

func (app *FinalityProviderApp) Stop() error {
	var stopErr error
	app.stopOnce.Do(func() {
		app.logger.Info("Stopping FinalityProviderApp")

		// Always stop the submission loop first to not generate additional events and actions
		app.logger.Debug("Stopping submission loop")
		close(app.quit)
		app.wg.Wait()

		app.logger.Debug("Stopping finality providers")
		if err := app.fpManager.Stop(); err != nil {
			stopErr = err
			return
		}

		app.logger.Debug("Sent to Babylon loop stopped")
		close(app.sentQuit)
		app.sentWg.Wait()

		app.logger.Debug("Stopping main eventLoop")
		close(app.eventQuit)
		app.eventWg.Wait()

		app.logger.Debug("Stopping EOTS manager")
		if err := app.eotsManager.Close(); err != nil {
			stopErr = err
			return
		}

		app.logger.Debug("FinalityProviderApp successfully stopped")

	})
	return stopErr
}

func (app *FinalityProviderApp) CreateFinalityProvider(
	keyName, chainID, passPhrase, hdPath string,
	description *stakingtypes.Description,
	commission *sdkmath.LegacyDec,
) (*CreateFinalityProviderResult, error) {

	req := &createFinalityProviderRequest{
		keyName:         keyName,
		chainID:         chainID,
		passPhrase:      passPhrase,
		hdPath:          hdPath,
		description:     description,
		commission:      commission,
		errResponse:     make(chan error, 1),
		successResponse: make(chan *createFinalityProviderResponse, 1),
	}

	app.createFinalityProviderRequestChan <- req

	select {
	case err := <-req.errResponse:
		return nil, err
	case successResponse := <-req.successResponse:
		return &CreateFinalityProviderResult{
			FpInfo: successResponse.FpInfo,
		}, nil
	case <-app.quit:
		return nil, fmt.Errorf("finality-provider app is shutting down")
	}
}

func (app *FinalityProviderApp) handleCreateFinalityProviderRequest(req *createFinalityProviderRequest) (*createFinalityProviderResponse, error) {
	// 1. check if the chain key exists
	kr, err := fpkr.NewChainKeyringControllerWithKeyring(app.kr, req.keyName, app.input)
	if err != nil {
		return nil, err
	}
	chainSk, err := kr.GetChainPrivKey(req.passPhrase)
	if err != nil {
		// the chain key does not exist, should create the chain key first
		keyInfo, err := kr.CreateChainKey(req.passPhrase, req.hdPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create chain key %s: %w", req.keyName, err)
		}
		chainSk = &secp256k1.PrivKey{Key: keyInfo.PrivateKey.Serialize()}
	}
	chainPk := &secp256k1.PubKey{Key: chainSk.PubKey().Bytes()}

	// 2. create EOTS key
	fpPkBytes, err := app.eotsManager.CreateKey(req.keyName, req.passPhrase, req.hdPath)
	if err != nil {
		return nil, err
	}
	fpPk, err := bbntypes.NewBIP340PubKey(fpPkBytes)
	if err != nil {
		return nil, err
	}
	fpRecord, err := app.eotsManager.KeyRecord(fpPk.MustMarshal(), req.passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get finality-provider record: %w", err)
	}

	// 3. create proof-of-possession
	pop, err := kr.CreatePop(fpRecord.PrivKey, req.passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create proof-of-possession of the finality-provider: %w", err)
	}

	if err := app.fps.CreateFinalityProvider(chainPk, fpPk.MustToBTCPK(), req.description, req.commission, req.keyName, req.chainID, pop.BabylonSig, pop.BtcSig); err != nil {
		return nil, fmt.Errorf("failed to save finality-provider: %w", err)
	}

	app.logger.Info("successfully created a finality-provider",
		zap.String("btc_pk", fpPk.MarshalHex()),
		zap.String("chain_pk", chainPk.String()),
		zap.String("key_name", req.keyName),
	)

	storedFp, err := app.fps.GetFinalityProvider(fpPk.MustToBTCPK())
	if err != nil {
		return nil, err
	}

	return &createFinalityProviderResponse{
		FpInfo: storedFp.ToFinalityProviderInfo(),
	}, nil
}

func CreateChainKey(keyringDir, chainID, keyName, backend, passphrase, hdPath string) (*types.ChainKeyInfo, error) {
	sdkCtx, err := fpkr.CreateClientCtx(
		keyringDir, chainID,
	)
	if err != nil {
		return nil, err
	}

	krController, err := fpkr.NewChainKeyringController(
		sdkCtx,
		keyName,
		backend,
	)
	if err != nil {
		return nil, err
	}

	return krController.CreateChainKey(passphrase, hdPath)
}

// main event loop for the finality-provider app
func (app *FinalityProviderApp) eventLoop() {
	defer app.eventWg.Done()

	for {
		select {
		case req := <-app.createFinalityProviderRequestChan:
			res, err := app.handleCreateFinalityProviderRequest(req)
			if err != nil {
				req.errResponse <- err
				continue
			}

			req.successResponse <- &createFinalityProviderResponse{FpInfo: res.FpInfo}

		case ev := <-app.finalityProviderRegisteredEventChan:
			// change the status of the finality-provider to registered
			err := app.fps.SetFpStatus(ev.btcPubKey.MustToBTCPK(), proto.FinalityProviderStatus_REGISTERED)
			if err != nil {
				app.logger.Fatal("failed to set finality-provider status to REGISTERED",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}

			// return to the caller
			ev.successResponse <- &RegisterFinalityProviderResponse{
				bbnPubKey: ev.bbnPubKey,
				btcPubKey: ev.btcPubKey,
				TxHash:    ev.txHash,
			}

		case <-app.eventQuit:
			app.logger.Debug("exiting main event loop")
			return
		}
	}
}

func (app *FinalityProviderApp) registrationLoop() {
	defer app.sentWg.Done()
	for {
		select {
		case req := <-app.registerFinalityProviderRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it fails due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			popBytes, err := req.pop.Marshal()
			if err != nil {
				req.errResponse <- err
				continue
			}

			desBytes, err := req.description.Marshal()
			if err != nil {
				req.errResponse <- err
				continue
			}
			res, err := app.cc.RegisterFinalityProvider(
				req.bbnPubKey.Key,
				req.btcPubKey.MustToBTCPK(),
				popBytes,
				req.commission,
				desBytes,
			)

			if err != nil {
				app.logger.Error(
					"failed to register finality-provider",
					zap.String("pk", req.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
				req.errResponse <- err
				continue
			}

			app.logger.Info(
				"successfully registered finality-provider on babylon",
				zap.String("btc_pk", req.btcPubKey.MarshalHex()),
				zap.String("babylon_pk", hex.EncodeToString(req.bbnPubKey.Key)),
				zap.String("txHash", res.TxHash),
			)

			app.finalityProviderRegisteredEventChan <- &finalityProviderRegisteredEvent{
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
