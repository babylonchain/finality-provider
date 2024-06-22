package e2etest_op

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bbncfg "github.com/babylonchain/babylon/client/config"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	e2etest "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type OpL2ConsumerTestManager struct {
	BabylonHandler   *e2etest.BabylonNodeHandler
	FpConfig         *fpcfg.Config
	BBNClient        *bbncc.BabylonController
	OpL2ConsumerCtrl *opstackl2.OPStackL2ConsumerController
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
	covenantPrivKeys, covenantPubKeys := e2etest.TenerateCovenantCommittee(numCovenants, t)

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

	ctm := &OpL2ConsumerTestManager{
		BabylonHandler: bh,
		FpConfig:       cfg,
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

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	err := ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(ctm.baseDir)
	require.NoError(t, err)
}
