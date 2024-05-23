package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

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
	"github.com/babylonchain/finality-provider/metrics"
	"github.com/babylonchain/finality-provider/types"
)

type FinalityProviderApp struct {
	startOnce sync.Once
	stopOnce  sync.Once

	wg   sync.WaitGroup
	quit chan struct{}

	cc          clientcontroller.ClientController
	consumerCon clientcontroller.ConsumerController
	kr          keyring.Keyring
	fps         *store.FinalityProviderStore
	config      *fpcfg.Config
	logger      *zap.Logger
	input       *strings.Reader

	fpManager   *FinalityProviderManager
	eotsManager eotsmanager.EOTSManager

	metrics *metrics.FpMetrics

	createFinalityProviderRequestChan   chan *createFinalityProviderRequest
	registerFinalityProviderRequestChan chan *registerFinalityProviderRequest
	finalityProviderRegisteredEventChan chan *finalityProviderRegisteredEvent
}

func NewFinalityProviderAppFromConfig(
	cfg *fpcfg.Config,
	db kvdb.Backend,
	logger *zap.Logger,
) (*FinalityProviderApp, error) {
	cc, err := clientcontroller.NewClientController(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client for the Babylon chain: %v", err)
	}
	consumerCon, err := clientcontroller.NewConsumerController(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client for the consumer chain %s: %v", cfg.ChainName, err)
	}
	// if the EOTSManagerAddress is empty, run a local EOTS manager;
	// otherwise connect a remote one with a gRPC client
	em, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create EOTS manager client: %w", err)
	}

	logger.Info("successfully connected to a remote EOTS manager", zap.String("address", cfg.EOTSManagerAddress))

	return NewFinalityProviderApp(cfg, cc, consumerCon, em, db, logger)
}

func NewFinalityProviderApp(
	config *fpcfg.Config,
	cc clientcontroller.ClientController,
	consumerCon clientcontroller.ConsumerController,
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

	fpMetrics := metrics.NewFpMetrics()

	fpm, err := NewFinalityProviderManager(fpStore, config, cc, consumerCon, em, fpMetrics, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality-provider manager: %w", err)
	}

	return &FinalityProviderApp{
		cc:                                  cc,
		consumerCon:                         consumerCon,
		fps:                                 fpStore,
		kr:                                  kr,
		config:                              config,
		logger:                              logger,
		input:                               input,
		fpManager:                           fpm,
		eotsManager:                         em,
		metrics:                             fpMetrics,
		quit:                                make(chan struct{}),
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

func (app *FinalityProviderApp) ListFinalityProviderInstancesForChain(chainID string) []*FinalityProviderInstance {
	return app.fpManager.ListFinalityProviderInstancesForChain(chainID)
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
		chainID:         fp.ChainID,
		bbnPubKey:       fp.ChainPk,
		btcPubKey:       bbntypes.NewBIP340PubKeyFromBTCPK(fp.BtcPk),
		pop:             pop,
		description:     fp.Description,
		commission:      fp.Commission,
		masterPubRand:   fp.MasterPubRand,
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

// SyncFinalityProviderStatus syncs the status of the finality-providers
func (app *FinalityProviderApp) SyncFinalityProviderStatus() error {
	latestBlockHeight, err := app.consumerCon.QueryLatestBlockHeight()
	if err != nil {
		return err
	}

	fps, err := app.fps.GetAllStoredFinalityProviders()
	if err != nil {
		return err
	}

	for _, fp := range fps {
		vp, err := app.consumerCon.QueryFinalityProviderVotingPower(fp.BtcPk, latestBlockHeight)
		if err != nil {
			// if error occured then the finality-provider is not registered in the Babylon chain yet
			continue
		}

		if vp > 0 {
			// voting power > 0 then set the status to ACTIVE
			err = app.fps.SetFpStatus(fp.BtcPk, proto.FinalityProviderStatus_ACTIVE)
			if err != nil {
				return err
			}
		} else if vp == 0 {
			// voting power == 0 then set status depending on previous status
			switch fp.Status {
			case proto.FinalityProviderStatus_CREATED:
				// previous status is CREATED then set to REGISTERED
				err = app.fps.SetFpStatus(fp.BtcPk, proto.FinalityProviderStatus_REGISTERED)
				if err != nil {
					return err
				}
			case proto.FinalityProviderStatus_ACTIVE:
				// previous status is ACTIVE then set to INACTIVE
				err = app.fps.SetFpStatus(fp.BtcPk, proto.FinalityProviderStatus_INACTIVE)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Start starts only the finality-provider daemon without any finality-provider instances
func (app *FinalityProviderApp) Start() error {
	var startErr error
	app.startOnce.Do(func() {
		app.logger.Info("Starting FinalityProviderApp")

		app.wg.Add(3)
		go app.eventLoop()
		go app.registrationLoop()
		go app.metricsUpdateLoop()
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
	storedFp, err := app.StoreFinalityProvider(req.keyName, req.passPhrase, req.hdPath, req.chainID, req.description, req.commission)
	if err != nil {
		return nil, err
	}

	return &createFinalityProviderResponse{
		FpInfo: storedFp.ToFinalityProviderInfo(),
	}, nil
}

// SignRawMsg loads the keyring private key and signs a message.
func (app *FinalityProviderApp) SignRawMsg(
	keyName, passPhrase, hdPath string,
	rawMsgToSign []byte,
) ([]byte, error) {
	_, chainSk, err := app.loadChainKeyring(keyName, passPhrase, hdPath)
	if err != nil {
		return nil, err
	}

	return chainSk.Sign(rawMsgToSign)
}

// loadChainKeyring checks the keyring by loading or creating a chain key.
func (app *FinalityProviderApp) loadChainKeyring(
	keyName, passPhrase, hdPath string,
) (*fpkr.ChainKeyringController, *secp256k1.PrivKey, error) {
	kr, err := fpkr.NewChainKeyringControllerWithKeyring(app.kr, keyName, app.input)
	if err != nil {
		return nil, nil, err
	}
	chainSk, err := kr.GetChainPrivKey(passPhrase)
	if err != nil {
		// the chain key does not exist, should create the chain key first
		keyInfo, err := kr.CreateChainKey(passPhrase, hdPath, "")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create chain key %s: %w", keyName, err)
		}
		chainSk = &secp256k1.PrivKey{Key: keyInfo.PrivateKey.Serialize()}
	}

	return kr, chainSk, nil
}

// StoreFinalityProvider stores a new finality provider in the fp store.
func (app *FinalityProviderApp) StoreFinalityProvider(
	keyName, passPhrase, hdPath, chainID string,
	description *stakingtypes.Description,
	commission *sdkmath.LegacyDec,
) (*store.StoredFinalityProvider, error) {
	// 1. check if the chain key exists
	kr, chainSk, err := app.loadChainKeyring(keyName, passPhrase, hdPath)
	if err != nil {
		return nil, err
	}
	chainPk := &secp256k1.PubKey{Key: chainSk.PubKey().Bytes()}

	// 2. create EOTS key
	fpPkBytes, err := app.eotsManager.CreateKey(keyName, passPhrase, hdPath)
	if err != nil {
		return nil, err
	}
	fpPk, err := bbntypes.NewBIP340PubKey(fpPkBytes)
	if err != nil {
		return nil, err
	}
	fpRecord, err := app.eotsManager.KeyRecord(fpPk.MustMarshal(), passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to get finality-provider record: %w", err)
	}

	// 3. create proof-of-possession
	pop, err := kr.CreatePop(fpRecord.PrivKey, passPhrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create proof-of-possession of the finality provider: %w", err)
	}

	// 4. Create derive master public randomness
	_, mpr, err := fpkr.GenerateMasterRandPair(fpRecord.PrivKey.Serialize(), types.MarshalChainID(chainID))
	if err != nil {
		return nil, fmt.Errorf("failed to get master public randomness of the finality provider: %w", err)
	}

	if err := app.fps.CreateFinalityProvider(chainPk, fpPk.MustToBTCPK(), description, commission, mpr.MarshalBase58(), keyName, chainID, pop.BabylonSig, pop.BtcSig); err != nil {
		return nil, fmt.Errorf("failed to save finality-provider: %w", err)
	}
	app.fpManager.metrics.RecordFpStatus(fpPk.MarshalHex(), proto.FinalityProviderStatus_CREATED)

	app.logger.Info("successfully created a finality-provider",
		zap.String("btc_pk", fpPk.MarshalHex()),
		zap.String("chain_pk", chainPk.String()),
		zap.String("key_name", keyName),
	)

	storedFp, err := app.fps.GetFinalityProvider(fpPk.MustToBTCPK())
	if err != nil {
		return nil, err
	}

	return storedFp, nil
}

func CreateChainKey(keyringDir, chainID, keyName, backend, passphrase, hdPath, mnemonic string) (*types.ChainKeyInfo, error) {
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

	return krController.CreateChainKey(passphrase, hdPath, mnemonic)
}

// main event loop for the finality-provider app
func (app *FinalityProviderApp) eventLoop() {
	defer app.wg.Done()

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
			btcPK := ev.btcPubKey.MustToBTCPK()
			// set the finality provider's registered epoch
			if err := app.fps.SetFpRegisteredEpoch(btcPK, ev.registeredEpoch); err != nil {
				app.logger.Fatal("failed to set the finality provider's registered epoch",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}
			// change the status of the finality-provider to registered
			if err := app.fps.SetFpStatus(btcPK, proto.FinalityProviderStatus_REGISTERED); err != nil {
				app.logger.Fatal("failed to set the finalityprovider's status to REGISTERED",
					zap.String("pk", ev.btcPubKey.MarshalHex()),
					zap.Error(err),
				)
			}
			app.fpManager.metrics.RecordFpStatus(ev.btcPubKey.MarshalHex(), proto.FinalityProviderStatus_REGISTERED)

			// return to the caller
			ev.successResponse <- &RegisterFinalityProviderResponse{
				bbnPubKey:       ev.bbnPubKey,
				btcPubKey:       ev.btcPubKey,
				TxHash:          ev.txHash,
				RegisteredEpoch: ev.registeredEpoch,
			}

		case <-app.quit:
			app.logger.Debug("exiting main event loop")
			return
		}
	}
}

func (app *FinalityProviderApp) registrationLoop() {
	defer app.wg.Done()
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
			res, registeredEpoch, err := app.cc.RegisterFinalityProvider(
				req.chainID,
				req.bbnPubKey.Key,
				req.btcPubKey.MustToBTCPK(),
				popBytes,
				req.commission,
				desBytes,
				req.masterPubRand,
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
				btcPubKey:       req.btcPubKey,
				bbnPubKey:       req.bbnPubKey,
				txHash:          res.TxHash,
				registeredEpoch: registeredEpoch,
				// pass the channel to the event so that we can send the response to the user which requested
				// the registration
				successResponse: req.successResponse,
			}
		case <-app.quit:
			app.logger.Debug("exiting registration loop")
			return
		}
	}
}

func (app *FinalityProviderApp) metricsUpdateLoop() {
	defer app.wg.Done()

	interval := app.config.Metrics.UpdateInterval
	app.logger.Info("starting metrics update loop",
		zap.Float64("interval seconds", interval.Seconds()))
	updateTicker := time.NewTicker(interval)

	for {
		select {
		case <-updateTicker.C:
			fps, err := app.fps.GetAllStoredFinalityProviders()
			if err != nil {
				app.logger.Error("failed to get finality-providers from the store", zap.Error(err))
				continue
			}
			app.metrics.UpdateFpMetrics(fps)
		case <-app.quit:
			updateTicker.Stop()
			app.logger.Info("exiting metrics update loop")
			return
		}
	}
}
