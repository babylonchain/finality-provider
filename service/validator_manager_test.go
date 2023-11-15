package service_test

import (
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	eventuallyWaitTimeOut = 1 * time.Second
	eventuallyPollTime    = 10 * time.Millisecond
)

func FuzzStatusUpdate(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		ctl := gomock.NewController(t)
		mockClientController := mocks.NewMockClientController(ctl)
		vm, cleanUp := newValidatorManagerWithRegisteredValidator(t, r, mockClientController)
		defer cleanUp()

		// setup mocks
		currentHeight := uint64(r.Int63n(100) + 1)
		currentBlockRes := &types.BlockInfo{
			Height: currentHeight,
			Hash:   datagen.GenRandomByteArray(r, 32),
		}
		mockClientController.EXPECT().QueryBestBlock().Return(currentBlockRes, nil).AnyTimes()
		mockClientController.EXPECT().Close().Return(nil).AnyTimes()
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()
		mockClientController.EXPECT().QueryBestBlock().Return(currentBlockRes, nil).AnyTimes()
		mockClientController.EXPECT().QueryActivatedHeight().Return(uint64(1), nil).AnyTimes()
		mockClientController.EXPECT().QueryBlock(gomock.Any()).Return(currentBlockRes, nil).AnyTimes()

		votingPower := uint64(r.Intn(2))
		mockClientController.EXPECT().QueryValidatorVotingPower(gomock.Any(), currentHeight).Return(votingPower, nil).AnyTimes()
		var slashedHeight uint64
		if votingPower == 0 {
			mockClientController.EXPECT().QueryValidatorSlashed(gomock.Any()).Return(true, nil).AnyTimes()
		}

		err := vm.Start()
		require.NoError(t, err)
		valIns := vm.ListValidatorInstances()[0]
		// stop the validator as we are testing static functionalities
		err = valIns.Stop()
		require.NoError(t, err)

		if votingPower > 0 {
			waitForStatus(t, valIns, proto.ValidatorStatus_ACTIVE)
		} else {
			if slashedHeight == 0 && valIns.GetStatus() == proto.ValidatorStatus_ACTIVE {
				waitForStatus(t, valIns, proto.ValidatorStatus_INACTIVE)
			} else if slashedHeight > 0 {
				waitForStatus(t, valIns, proto.ValidatorStatus_SLASHED)
			}
		}
	})
}

func waitForStatus(t *testing.T, valIns *service.ValidatorInstance, s proto.ValidatorStatus) {
	require.Eventually(t,
		func() bool {
			return valIns.GetStatus() == s
		}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func newValidatorManagerWithRegisteredValidator(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController) (*service.ValidatorManager, func()) {
	// create validator app with config
	cfg := valcfg.DefaultConfig()
	cfg.StatusUpdateInterval = 10 * time.Millisecond
	cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
	cfg.BabylonConfig.KeyDirectory = t.TempDir()
	logger := logrus.New()

	input := strings.NewReader("")
	kr, err := service.CreateKeyring(
		cfg.BabylonConfig.KeyDirectory,
		cfg.BabylonConfig.ChainID,
		cfg.BabylonConfig.KeyringBackend,
		input,
	)
	require.NoError(t, err)

	valStore, err := val.NewValidatorStore(cfg.DatabaseConfig)
	require.NoError(t, err)

	eotsCfg, err := valcfg.NewEOTSManagerConfigFromAppConfig(&cfg)
	require.NoError(t, err)
	em, err := eotsmanager.NewLocalEOTSManager(eotsCfg, logger)
	require.NoError(t, err)

	vm, err := service.NewValidatorManager(valStore, &cfg, cc, em, logger)
	require.NoError(t, err)

	// create registered validator
	keyName := datagen.GenRandomHexStr(r, 10)
	chainID := datagen.GenRandomHexStr(r, 10)
	kc, err := val.NewChainKeyringControllerWithKeyring(kr, keyName, input)
	require.NoError(t, err)
	btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
	require.NoError(t, err)
	btcPk, err := bbntypes.NewBIP340PubKey(btcPkBytes)
	require.NoError(t, err)
	bbnPk, err := kc.CreateChainKey(passphrase, hdPath)
	require.NoError(t, err)
	valRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
	require.NoError(t, err)
	pop, err := kc.CreatePop(valRecord.PrivKey, passphrase)
	require.NoError(t, err)

	storedValidator := val.NewStoreValidator(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())
	storedValidator.Status = proto.ValidatorStatus_REGISTERED
	err = valStore.SaveValidator(storedValidator)
	require.NoError(t, err)

	cleanUp := func() {
		err = vm.Stop()
		require.NoError(t, err)
		err := os.RemoveAll(cfg.DatabaseConfig.Path)
		require.NoError(t, err)
		err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
		require.NoError(t, err)
		err = os.RemoveAll(cfg.EOTSManagerConfig.DBPath)
		require.NoError(t, err)
	}

	return vm, cleanUp
}
