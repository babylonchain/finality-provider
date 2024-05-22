package service_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager"
	eotscfg "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/babylonchain/finality-provider/keyring"
	fpkr "github.com/babylonchain/finality-provider/keyring"
	"github.com/babylonchain/finality-provider/metrics"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/babylonchain/finality-provider/testutil/mocks"
	"github.com/babylonchain/finality-provider/types"
	"github.com/babylonchain/finality-provider/util"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
		mockClientController := testutil.PrepareMockedBabylonController(t, uint64(0))
		mockConsumerController := mocks.NewMockConsumerController(ctl)
		vm, fpPk, cleanUp := newFinalityProviderManagerWithRegisteredFp(t, r, mockClientController, mockConsumerController)
		defer cleanUp()

		// setup mocks
		currentHeight := uint64(r.Int63n(100) + 1)
		currentBlockRes := &types.BlockInfo{
			Height: currentHeight,
			Hash:   datagen.GenRandomByteArray(r, 32),
		}
		mockConsumerController.EXPECT().Close().Return(nil).AnyTimes()
		mockConsumerController.EXPECT().QueryLatestFinalizedBlock().Return(nil, nil).AnyTimes()
		mockConsumerController.EXPECT().QueryLatestBlockHeight().Return(currentHeight, nil).AnyTimes()
		mockConsumerController.EXPECT().QueryActivatedHeight().Return(uint64(1), nil).AnyTimes()
		mockConsumerController.EXPECT().QueryBlock(gomock.Any()).Return(currentBlockRes, nil).AnyTimes()

		votingPower := uint64(r.Intn(2))
		mockConsumerController.EXPECT().QueryFinalityProviderVotingPower(gomock.Any(), currentHeight).Return(votingPower, nil).AnyTimes()
		mockConsumerController.EXPECT().SubmitFinalitySig(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.TxResponse{TxHash: ""}, nil).AnyTimes()
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

func newFinalityProviderManagerWithRegisteredFp(t *testing.T, r *rand.Rand, cc clientcontroller.ClientController, consumerCon clientcontroller.ConsumerController) (*service.FinalityProviderManager, *bbntypes.BIP340PubKey, func()) {
	logger := zap.NewNop()
	// create an EOTS manager
	eotsHomeDir := filepath.Join(t.TempDir(), "eots-home")
	eotsCfg := eotscfg.DefaultConfigWithHomePath(eotsHomeDir)
	eotsdb, err := eotsCfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	em, err := eotsmanager.NewLocalEOTSManager(eotsHomeDir, eotsCfg.KeyringBackend, eotsdb, logger)
	require.NoError(t, err)

	// create finality-provider app with randomized config
	fpHomeDir := filepath.Join(t.TempDir(), "fp-home")
	fpCfg := fpcfg.DefaultConfigWithHome(fpHomeDir)
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
	fpdb, err := fpCfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpStore, err := fpstore.NewFinalityProviderStore(fpdb)
	require.NoError(t, err)

	metricsCollectors := metrics.NewFpMetrics()
	vm, err := service.NewFinalityProviderManager(fpStore, &fpCfg, cc, consumerCon, em, metricsCollectors, logger)
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
	keyInfo, err := kc.CreateChainKey(passphrase, hdPath, "")
	require.NoError(t, err)
	bbnPk := &secp256k1.PubKey{Key: keyInfo.PublicKey.SerializeCompressed()}
	fpRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
	require.NoError(t, err)
	pop, err := kc.CreatePop(fpRecord.PrivKey, passphrase)
	require.NoError(t, err)

	_, mpr, err := fpkr.GenerateMasterRandPair(fpRecord.PrivKey.Serialize(), types.MarshalChainID(chainID))
	require.NoError(t, err)

	err = fpStore.CreateFinalityProvider(
		bbnPk,
		btcPk.MustToBTCPK(),
		testutil.RandomDescription(r),
		testutil.ZeroCommissionRate(),
		mpr.MarshalBase58(),
		keyName,
		chainID,
		pop.BabylonSig,
		pop.BtcSig,
	)
	require.NoError(t, err)

	err = fpStore.SetFpStatus(btcPk.MustToBTCPK(), proto.FinalityProviderStatus_REGISTERED)
	require.NoError(t, err)

	err = fpStore.SetFpRegisteredEpoch(btcPk.MustToBTCPK(), 0)
	require.NoError(t, err)

	cleanUp := func() {
		err = vm.Stop()
		require.NoError(t, err)
		err = eotsdb.Close()
		require.NoError(t, err)
		err = fpdb.Close()
		require.NoError(t, err)
		err = os.RemoveAll(eotsHomeDir)
		require.NoError(t, err)
		err = os.RemoveAll(fpHomeDir)
		require.NoError(t, err)
	}

	return vm, btcPk, cleanUp
}
