package service_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/util"

	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/keyring"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"

	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/testutil/mocks"
	"github.com/babylonchain/finality-provider/types"
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
		vm, fpPk, cleanUp := newFinalityProviderManagerWithRegisteredFp(t, r, mockClientController)
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
		mockClientController.EXPECT().QueryFinalityProviderVotingPower(gomock.Any(), currentHeight).Return(votingPower, nil).AnyTimes()
		var slashedHeight uint64
		if votingPower == 0 {
			mockClientController.EXPECT().QueryFinalityProviderSlashed(gomock.Any()).Return(true, nil).AnyTimes()
		}

		err := vm.StartFinalityProvider(fpPk, passphrase)
		require.NoError(t, err)
		fpIns := vm.ListFinalityProviderInstances()[0]
		// stop the finality-provider as we are testing static functionalities
		err = fpIns.Stop()
		require.NoError(t, err)

		if votingPower > 0 {
			waitForStatus(t, fpIns, proto.FinalityProviderStatus_ACTIVE)
		} else {
			if slashedHeight == 0 && fpIns.GetStatus() == proto.FinalityProviderStatus_ACTIVE {
				waitForStatus(t, fpIns, proto.FinalityProviderStatus_INACTIVE)
			} else if slashedHeight > 0 {
				waitForStatus(t, fpIns, proto.FinalityProviderStatus_SLASHED)
			}
		}
	})
}

func waitForStatus(t *testing.T, fpIns *service.FinalityProviderInstance, s proto.FinalityProviderStatus) {
	require.Eventually(t,
		func() bool {
			return fpIns.GetStatus() == s
		}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func newFinalityProviderManagerWithRegisteredFp(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController) (*service.FinalityProviderManager, *bbntypes.BIP340PubKey, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := testutil.GenEOTSConfig(r, t)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg, logger)
	require.NoError(t, err)

	// create finality-provider app with randomized config
	fpHomeDir := filepath.Join(t.TempDir(), "fp-home")
	fpCfg := testutil.GenFpConfig(r, t, fpHomeDir)
	fpCfg.StatusUpdateInterval = 10 * time.Millisecond
	input := strings.NewReader("")
	kr, err := keyring.CreateKeyring(
		fpCfg.BabylonConfig.KeyDirectory,
		fpCfg.BabylonConfig.ChainID,
		fpCfg.BabylonConfig.KeyringBackend,
		input,
	)
	require.NoError(t, err)
	err = util.MakeDirectory(fpcfg.DataDir(fpHomeDir))
	require.NoError(t, err)
	fpStore, err := fpstore.NewFinalityProviderStore(
		fpcfg.DBPath(fpHomeDir),
		fpCfg.DatabaseConfig.Name,
		fpCfg.DatabaseConfig.Backend,
	)
	require.NoError(t, err)

	vm, err := service.NewFinalityProviderManager(fpStore, fpCfg, cc, em, logger)
	require.NoError(t, err)

	// create registered finality-provider
	keyName := datagen.GenRandomHexStr(r, 10)
	chainID := datagen.GenRandomHexStr(r, 10)
	kc, err := keyring.NewChainKeyringControllerWithKeyring(kr, keyName, input)
	require.NoError(t, err)
	btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
	require.NoError(t, err)
	btcPk, err := bbntypes.NewBIP340PubKey(btcPkBytes)
	require.NoError(t, err)
	keyInfo, err := kc.CreateChainKey(passphrase, hdPath)
	require.NoError(t, err)
	bbnPk := &secp256k1.PubKey{Key: keyInfo.PublicKey.SerializeCompressed()}
	fpRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
	require.NoError(t, err)
	pop, err := kc.CreatePop(fpRecord.PrivKey, passphrase)
	require.NoError(t, err)

	storedFp := fpstore.NewStoreFinalityProvider(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())
	storedFp.Status = proto.FinalityProviderStatus_REGISTERED
	err = fpStore.SaveFinalityProvider(storedFp)
	require.NoError(t, err)

	cleanUp := func() {
		err = vm.Stop()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(fpHomeDir)
		require.NoError(t, err)
	}

	return vm, btcPk, cleanUp
}
