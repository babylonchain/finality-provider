package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/babylonchain/finality-provider/types"
	"github.com/babylonchain/finality-provider/util"

	sdkmath "cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"go.uber.org/zap"

	fpkr "github.com/babylonchain/finality-provider/keyring"

	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/proto"

	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"
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
	fps    *fpstore.FinalityProviderStore
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
	homePath string,
	config *fpcfg.Config,
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
	// TODO add retry mechanism and ping to ensure the EOTS manager daemon is healthy
	logger.Info("successfully connected to a remote EOTS manager", zap.String("address", config.EOTSManagerAddress))

	return NewFinalityProviderApp(homePath, config, cc, em, logger)
}

func NewFinalityProviderApp(
	homePath string,
	config *fpcfg.Config,
	cc clientcontroller.ClientController,
	em eotsmanager.EOTSManager,
	logger *zap.Logger,
) (*FinalityProviderApp, error) {
	fpStore, err := initStore(homePath, config)
	if err != nil {
		return nil, fmt.Errorf("failed to load store: %w", err)
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

func initStore(homePath string, cfg *fpcfg.Config) (*fpstore.FinalityProviderStore, error) {
	// Create the directory that will store the data
	if err := util.MakeDirectory(fpcfg.DataDir(homePath)); err != nil {
		return nil, err
	}

	return fpstore.NewFinalityProviderStore(fpcfg.DBPath(homePath), cfg.DatabaseConfig.Name, cfg.DatabaseConfig.Backend)
}

func (app *FinalityProviderApp) GetConfig() *fpcfg.Config {
	return app.config
}

func (app *FinalityProviderApp) GetFinalityProviderStore() *fpstore.FinalityProviderStore {
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

// GetFinalityProviderInstance returns the finality-provider instance with the given Babylon public key
func (app *FinalityProviderApp) GetFinalityProviderInstance(fpPk *bbntypes.BIP340PubKey) (*FinalityProviderInstance, error) {
	return app.fpManager.GetFinalityProviderInstance(fpPk)
}

func (app *FinalityProviderApp) RegisterFinalityProvider(fpPkStr string) (*RegisterFinalityProviderResponse, error) {
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(fpPkStr)
	if err != nil {
		return nil, err
	}

	fp, err := app.fps.GetStoreFinalityProvider(fpPk.MustMarshal())
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
		BabylonSig: fp.Pop.BabylonSig,
		BtcSig:     btcSig.MustMarshal(),
		BtcSigType: bstypes.BTCSigType_BIP340,
	}

	commissionRate, err := sdkmath.LegacyNewDecFromStr(fp.Commission)
	if err != nil {
		return nil, err
	}

	request := &registerFinalityProviderRequest{
		bbnPubKey:       fp.GetBabylonPK(),
		btcPubKey:       fp.MustGetBIP340BTCPK(),
		pop:             pop,
		description:     fp.Description,
		commission:      &commissionRate,
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

// NOTE: this is not safe in production, so only used for testing purpose
func (app *FinalityProviderApp) getFpPrivKey(fpPk []byte) (*btcec.PrivateKey, error) {
	record, err := app.eotsManager.KeyRecord(fpPk, "")
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
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

		// Closing db as last to avoid anybody to write do db
		app.logger.Debug("Stopping data store")
		if err := app.fps.Close(); err != nil {
			stopErr = err
			return
		}

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
	description []byte,
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
			FpPk: successResponse.FpPk,
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

	fp := fpstore.NewStoreFinalityProvider(chainPk, fpPk, req.keyName, req.chainID, pop, req.description, req.commission)

	if err := app.fps.SaveFinalityProvider(fp); err != nil {
		return nil, fmt.Errorf("failed to save finality-provider: %w", err)
	}

	app.logger.Info("successfully created a finality-provider",
		zap.String("btc_pk", fpPk.MarshalHex()),
		zap.String("chain_pk", chainPk.String()),
		zap.String("key_name", req.keyName),
	)

	return &createFinalityProviderResponse{
		FpPk: fpPk,
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

			req.successResponse <- &createFinalityProviderResponse{FpPk: res.FpPk}

		case ev := <-app.finalityProviderRegisteredEventChan:
			fpStored, err := app.fps.GetStoreFinalityProvider(ev.btcPubKey.MustMarshal())
			if err != nil {
				// we always check if the finality-provider is in the DB before sending the registration request
				app.logger.Fatal(
					"registered finality-provider not found in DB",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}

			// change the status of the finality-provider to registered
			err = app.fps.SetFinalityProviderStatus(fpStored, proto.FinalityProviderStatus_REGISTERED)
			if err != nil {
				app.logger.Fatal("failed to set finality-provider status to REGISTERED",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}

			// return to the caller
			ev.successResponse <- &RegisterFinalityProviderResponse{
				bbnPubKey: fpStored.GetBabylonPK(),
				btcPubKey: fpStored.MustGetBIP340BTCPK(),
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

			res, err := app.cc.RegisterFinalityProvider(
				req.bbnPubKey.Key,
				req.btcPubKey.MustToBTCPK(),
				popBytes,
				req.commission,
				req.description,
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
