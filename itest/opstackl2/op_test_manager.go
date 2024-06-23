package e2etest_op

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	bbncfg "github.com/babylonchain/babylon/client/config"
	bbntypes "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	e2etest "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type OpL2ConsumerTestManager struct {
	BabylonHandler    *e2etest.BabylonNodeHandler
	EOTSServerHandler *e2etest.EOTSServerHandler
	FpConfig          *fpcfg.Config
	EOTSConfig        *eotsconfig.Config
	FpApp             *service.FinalityProviderApp
	EOTSClient        *client.EOTSManagerGRpcClient
	BBNClient         *bbncc.BabylonController
	OpL2ConsumerCtrl  *opstackl2.OPStackL2ConsumerController
	// TODO: not sure if needed, if not can remove
	StakingParams    *types.StakingParams
	CovenantPrivKeys []*btcec.PrivateKey
	baseDir          string
}

func StartOpL2ConsumerManager(t *testing.T) *OpL2ConsumerTestManager {
	// Setup consumer test manager
	testDir, err := e2etest.BaseDir("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2etest.GenerateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := e2etest.NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := e2etest.DefaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. new op consumer controller
	opcc, err := opstackl2.NewOPStackL2ConsumerController(mockOpL2ConsumerCtrlConfig(bh.GetNodeDataDir()), logger)
	require.NoError(t, err)

	// 4. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := e2etest.NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 5. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, opcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	ctm := &OpL2ConsumerTestManager{
		BabylonHandler:    bh,
		EOTSServerHandler: eh,
		FpConfig:          cfg,
		EOTSConfig:        eotsCfg,
		FpApp:             fpApp,
		EOTSClient:        eotsCli,
		// TODO: might not need this bc field
		BBNClient:        bc,
		OpL2ConsumerCtrl: opcc,
		CovenantPrivKeys: covenantPrivKeys,
		baseDir:          testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func mockOpL2ConsumerCtrlConfig(nodeDataDir string) *fpcfg.OPStackL2Config {
	dc := bbncfg.DefaultBabylonConfig()

	// fill up the config from dc config
	return &fpcfg.OPStackL2Config{
		// it has to be a valid EVM RPC which doesn't timeout
		OPStackL2RPCAddress: "https://rpc.ankr.com/eth",
		// it has to be a valid addr that can be passed into `sdktypes.AccAddressFromBech32()`
		OPFinalityGadgetAddress: "bbn1ghd753shjuwexxywmgs4xz7x2q732vcnkm6h2pyv9s6ah3hylvrqxxvh0f",
		Key:                     dc.Key,
		ChainID:                 dc.ChainID,
		RPCAddr:                 dc.RPCAddr,
		GRPCAddr:                dc.GRPCAddr,
		AccountPrefix:           dc.AccountPrefix,
		KeyringBackend:          dc.KeyringBackend,
		KeyDirectory:            nodeDataDir,
		GasAdjustment:           1.5,
		GasPrices:               "0.002ubbn",
		Debug:                   dc.Debug,
		Timeout:                 dc.Timeout,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: dc.OutputFormat,
		SignModeStr:  dc.SignModeStr,
	}
}

func (ctm *OpL2ConsumerTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, e2etest.EventuallyWaitTimeOut, e2etest.EventuallyPollTime)
	t.Logf("Babylon node is started")
}

func (ctm *OpL2ConsumerTestManager) StartFinalityProvider(t *testing.T, n int) []*service.FinalityProviderInstance {
	app := ctm.FpApp

	for i := 0; i < n; i++ {
		fpName := e2etest.FpNamePrefix + strconv.Itoa(i)
		moniker := e2etest.MonikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := e2etest.NewDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, e2etest.Passphrase, e2etest.HdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, e2etest.ChainID, e2etest.Passphrase, e2etest.HdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
		require.NoError(t, err)
		err = app.StartHandlingFinalityProvider(fpPk, e2etest.Passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPk)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)

		// check finality providers on Babylon side
		require.Eventually(t, func() bool {
			fps, err := ctm.BBNClient.QueryFinalityProviders()
			if err != nil {
				t.Logf("failed to query finality providers from Babylon %s", err.Error())
				return false
			}

			if len(fps) != i+1 {
				return false
			}

			for _, fp := range fps {
				if !strings.Contains(fp.Description.Moniker, e2etest.MonikerPrefix) {
					return false
				}
				if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
					return false
				}
			}

			return true
		}, e2etest.EventuallyWaitTimeOut, e2etest.EventuallyPollTime)
	}

	fpInsList := app.ListFinalityProviderInstances()
	require.Equal(t, n, len(fpInsList))

	t.Logf("The test manager is running with %v finality-provider(s)", len(fpInsList))

	for i, fp := range fpInsList {
		t.Logf("The num %d finality-provider pubkeyhex %s", i, fp.GetBtcPkHex())
	}

	return fpInsList
}

func (ctm *OpL2ConsumerTestManager) generateCommitPubRandListMsg(fpPk *bbntypes.BIP340PubKey, startHeight uint64, numPubRand uint64) (*ftypes.MsgCommitPubRandList, error) {
	// generate a list of Schnorr randomness pairs
	fp, err := ctm.FpApp.GetFinalityProviderInstance(fpPk)
	if err != nil {
		return nil, fmt.Errorf("failed to get fp instance: %w", err)
	}
	pubRandList, err := fp.GetPubRandList(startHeight, numPubRand)
	if err != nil {
		return nil, fmt.Errorf("failed to generate randomness: %w", err)
	}
	// generate commitment and proof for each public randomness
	commitment, proofList := types.GetPubRandCommitAndProofs(pubRandList)

	// store them to database
	if err := ctm.FpApp.GetPubRandProofStore().AddPubRandProofList(pubRandList, proofList); err != nil {
		return nil, fmt.Errorf("failed to save public randomness to DB: %w", err)
	}

	// sign the commitment
	schnorrSig, err := fp.SignPubRandCommit(startHeight, numPubRand, commitment)
	if err != nil {
		return nil, fmt.Errorf("failed to sign the Schnorr signature: %w", err)
	}

	msg := &ftypes.MsgCommitPubRandList{
		Signer:      ctm.BBNClient.MustGetTxSigner(),
		FpBtcPk:     fpPk,
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  commitment,
		Sig:         bbntypes.NewBIP340SignatureFromBTCSig(schnorrSig),
	}
	return msg, nil
}

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	var err error
	// FpApp has to stop first or you will get "rpc error: desc = account xxx not found: key not found" error
	// b/c when Babylon daemon is stopped, FP won't be able to find the keyring backend
	err = ctm.FpApp.Stop()
	require.NoError(t, err)
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	err = os.RemoveAll(ctm.baseDir)
	require.NoError(t, err)
}
