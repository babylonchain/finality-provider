package e2etest

import (
	"path/filepath"
	"testing"

	sdklogs "cosmossdk.io/log"
	wasmapp "github.com/CosmWasm/wasmd/app"
	wasmparams "github.com/CosmWasm/wasmd/app/params"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	fpcc "github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type ConsumerTestManager struct {
	BabylonHandler      *BabylonNodeHandler
	FpConfig            *fpcfg.Config
	BBNClient           *fpcc.BabylonController
	WasmdHandler        *WasmdNodeHandler
	WasmdConsumerClient *fpcc.WasmdConsumerController
	StakingParams       *types.StakingParams
	baseDir             string
}

func StartConsumerManager(t *testing.T) *ConsumerTestManager {
	// Setup test manager
	testDir, err := tempDirWithName("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	_, covenantPubKeys := generateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := defaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := fpcc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	wh := NewWasmdNodeHandler(t)
	err = wh.Start()
	require.NoError(t, err)
	// Setup wasmd consumer client
	cfg.WasmdConfig = config.DefaultWasmdConfig()
	cfg.WasmdConfig.KeyDirectory = wh.dataDir
	cfg.ChainName = fpcc.WasmdConsumerChainName
	//encodingConfig := wasmapp.MakeEncodingConfig(t)
	tempApp := wasmapp.NewWasmApp(sdklogs.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(t.TempDir()), []wasmkeeper.Option{})
	encodingConfig := wasmparams.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	wcc, err := fpcc.NewWasmdConsumerController(cfg.WasmdConfig, encodingConfig, logger)
	require.NoError(t, err)

	ctm := &ConsumerTestManager{
		BabylonHandler:      bh,
		FpConfig:            cfg,
		BBNClient:           bc,
		WasmdHandler:        wh,
		WasmdConsumerClient: wcc,
		baseDir:             testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func (ctm *ConsumerTestManager) WaitForServicesStart(t *testing.T) {
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
		blockHeight, err := ctm.WasmdHandler.GetLatestBlockHeight()
		if err != nil {
			t.Logf("failed to get latest block height from wasmd %s", err.Error())
			return false
		}
		return blockHeight > 2
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Wasmd node is started")
}

func (ctm *ConsumerTestManager) Stop(t *testing.T) {
	ctm.WasmdHandler.Stop(t)
	err := ctm.BabylonHandler.Stop()
	require.NoError(t, err)
}
