package e2etest

import (
	"os"
	"path/filepath"
	"testing"

	sdklogs "cosmossdk.io/log"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/babylonchain/babylon/testutil/datagen"
	fpcc "github.com/babylonchain/finality-provider/clientcontroller"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	cwcc "github.com/babylonchain/finality-provider/clientcontroller/cosmwasm"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type BcdTestManager struct {
	BabylonHandler    *BabylonNodeHandler
	FpConfig          *fpcfg.Config
	BBNClient         *bbncc.BabylonController
	BcdHandler        *BcdNodeHandler
	BcdConsumerClient *cwcc.CosmwasmConsumerController
	StakingParams     *types.StakingParams
	EOTSServerHandler *EOTSServerHandler
	EOTSConfig        *eotsconfig.Config
	Fpa               *service.FinalityProviderApp
	EOTSClient        *client.EOTSManagerGRpcClient
	CovenantPrivKeys  []*btcec.PrivateKey
	baseDir           string
}

func StartBcdTestManager(t *testing.T) *BcdTestManager {
	// Setup consumer test manager
	testDir, err := tempDirWithName("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := generateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := defaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. setup wasmd node
	wh := NewBcdNodeHandler(t)
	err = wh.Start()
	require.NoError(t, err)
	cfg.CosmwasmConfig = config.DefaultCosmwasmConfig()
	cfg.CosmwasmConfig.KeyDirectory = wh.dataDir
	// TODO: make random contract addresses for now to avoid validation errors
	//  later in the e2e tests we would upload the contract and update the addresses
	//  investigate if there is a better way to handle this
	cfg.CosmwasmConfig.BabylonContractAddress = datagen.GenRandomAccount().GetAddress().String()
	cfg.CosmwasmConfig.BtcStakingContractAddress = datagen.GenRandomAccount().GetAddress().String()
	cfg.ChainName = fpcc.WasmdConsumerChainName
	tempApp := wasmapp.NewWasmApp(sdklogs.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(t.TempDir()), []wasmkeeper.Option{})
	encodingCfg := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	wcc, err := cwcc.NewCosmwasmConsumerController(cfg.CosmwasmConfig, encodingCfg, logger)
	require.NoError(t, err)

	// 4. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 5. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, wcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	// TODO: setup fp app after contract supports relevant queries

	ctm := &BcdTestManager{
		BabylonHandler:    bh,
		FpConfig:          cfg,
		BBNClient:         bc,
		BcdHandler:        wh,
		BcdConsumerClient: wcc,
		EOTSServerHandler: eh,
		EOTSConfig:        eotsCfg,
		Fpa:               fpApp,
		EOTSClient:        eotsCli,
		CovenantPrivKeys:  covenantPrivKeys,
		baseDir:           testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func (ctm *BcdTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Babylon node is started")

	// wait for wasmd to start
	require.Eventually(t, func() bool {
		blockHeight, err := ctm.BcdHandler.GetLatestBlockHeight()
		if err != nil {
			t.Logf("failed to get latest block height from wasmd %s", err.Error())
			return false
		}
		return blockHeight > 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Bcd node is started")
}

func (ctm *BcdTestManager) Stop(t *testing.T) {
	err := ctm.Fpa.Stop()
	require.Error(t, err) // TODO: expect error for now as finality daemon is not started in tests
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	ctm.BcdHandler.Stop(t)
	err = os.RemoveAll(ctm.baseDir)
	require.NoError(t, err)
}
