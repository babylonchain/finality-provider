package service_test

import (
	"github.com/babylonchain/btc-validator/util"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/keyring"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/validator/proto"
	"github.com/babylonchain/btc-validator/validator/service"
	valstore "github.com/babylonchain/btc-validator/validator/store"
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
		vm, valPk, cleanUp := newValidatorManagerWithRegisteredValidator(t, r, mockClientController)
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

		err := vm.StartValidator(valPk, passphrase)
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

func newValidatorManagerWithRegisteredValidator(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController) (*service.ValidatorManager, *bbntypes.BIP340PubKey, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := testutil.GenEOTSConfig(r, t)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg, logger)
	require.NoError(t, err)

	// create validator app with randomized config
	valHomeDir := filepath.Join(t.TempDir(), "val-home")
	valCfg := testutil.GenValConfig(r, t, valHomeDir)
	valCfg.StatusUpdateInterval = 10 * time.Millisecond
	input := strings.NewReader("")
	kr, err := keyring.CreateKeyring(
		valCfg.BabylonConfig.KeyDirectory,
		valCfg.BabylonConfig.ChainID,
		valCfg.BabylonConfig.KeyringBackend,
		input,
	)
	require.NoError(t, err)
	err = util.MakeDirectory(valcfg.DataDir(valHomeDir))
	require.NoError(t, err)
	valStore, err := valstore.NewValidatorStore(
		valcfg.DBPath(valHomeDir),
		valCfg.DatabaseConfig.Name,
		valCfg.DatabaseConfig.Backend,
	)
	require.NoError(t, err)

	vm, err := service.NewValidatorManager(valStore, valCfg, cc, em, logger)
	require.NoError(t, err)

	// create registered validator
	keyName := datagen.GenRandomHexStr(r, 10)
	chainID := datagen.GenRandomHexStr(r, 10)
	kc, err := keyring.NewChainKeyringControllerWithKeyring(kr, keyName, input)
	require.NoError(t, err)
	btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
	require.NoError(t, err)
	btcPk, err := bbntypes.NewBIP340PubKey(btcPkBytes)
	require.NoError(t, err)
	keyPair, err := kc.CreateChainKey(passphrase, hdPath)
	require.NoError(t, err)
	bbnPk := &secp256k1.PubKey{Key: keyPair.PublicKey.SerializeCompressed()}
	valRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
	require.NoError(t, err)
	pop, err := kc.CreatePop(valRecord.PrivKey, passphrase)
	require.NoError(t, err)

	storedValidator := valstore.NewStoreValidator(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())
	storedValidator.Status = proto.ValidatorStatus_REGISTERED
	err = valStore.SaveValidator(storedValidator)
	require.NoError(t, err)

	cleanUp := func() {
		err = vm.Stop()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(valHomeDir)
		require.NoError(t, err)
	}

	return vm, btcPk, cleanUp
}
