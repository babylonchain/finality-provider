package e2etest

import (
	"os"
	"path/filepath"
	"testing"

	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type OpL2ConsumerTestManager struct {
	BabylonHandler   *BabylonNodeHandler
	FpConfig         *fpcfg.Config
	BBNClient        *bbncc.BabylonController
	OpL2Consumer     *opstackl2.OPStackL2ConsumerController
	StakingParams    *types.StakingParams
	CovenantPrivKeys []*btcec.PrivateKey
	baseDir          string
}

func StartOpL2ConsumerManager(t *testing.T) *OpL2ConsumerTestManager {
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

	ctm := &OpL2ConsumerTestManager{
		BabylonHandler: bh,
		FpConfig:       cfg,
		// TODO: might not need this bc field
		BBNClient:        bc,
		CovenantPrivKeys: covenantPrivKeys,
		baseDir:          testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func (ctm *OpL2ConsumerTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("Babylon node is started")
}

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	err := ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(ctm.baseDir)
	require.NoError(t, err)
}
