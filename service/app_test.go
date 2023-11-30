package service_test

import (
	"math/rand"
	"os"
	"testing"

	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	passphrase = "testpass"
	hdPath     = ""
)

func FuzzRegisterValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.EOTSManagerConfig.DBPath)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
			require.NoError(t, err)
		}()
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		cfg.ValidatorModeConfig.AutoChainScanningMode = false
		cfg.ValidatorModeConfig.StaticChainScanningStartHeight = randomStartingHeight
		currentHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight, &types.StakingParams{})
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()
		eotsCfg, err := valcfg.NewEOTSManagerConfigFromAppConfig(&cfg)
		require.NoError(t, err)
		logger := logrus.New()
		em, err := eotsmanager.NewLocalEOTSManager(eotsCfg, logger)
		require.NoError(t, err)
		app, err := service.NewValidatorApp(&cfg, mockClientController, em, logger)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := testutil.GenStoredValidator(r, t, app, passphrase, hdPath)
		btcSig := new(bbntypes.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		pop := &bstypes.ProofOfPossession{
			BabylonSig: validator.Pop.BabylonSig,
			BtcSig:     btcSig.MustMarshal(),
			BtcSigType: bstypes.BTCSigType_BIP340,
		}
		popBytes, err := pop.Marshal()
		require.NoError(t, err)

		txHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().
			RegisterValidator(
				validator.GetBabylonPK().Key,
				validator.MustGetBIP340BTCPK().MustToBTCPK(),
				popBytes,
				testutil.ZeroCommissionRate().BigInt(),
				testutil.EmptyDescription().String(),
			).Return(&types.TxResponse{TxHash: txHash}, nil).AnyTimes()

		res, err := app.RegisterValidator(validator.MustGetBIP340BTCPK().MarshalHex())
		require.NoError(t, err)
		require.Equal(t, txHash, res.TxHash)

		err = app.StartHandlingValidator(validator.MustGetBIP340BTCPK(), passphrase)
		require.NoError(t, err)

		valAfterReg, err := app.GetValidatorInstance(validator.MustGetBIP340BTCPK())
		require.NoError(t, err)
		require.Equal(t, valAfterReg.GetStoreValidator().Status, proto.ValidatorStatus_REGISTERED)
	})
}
