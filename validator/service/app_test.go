package service_test

import (
	"math/rand"
	"os"
	"testing"

	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/validator/proto"
	"github.com/babylonchain/btc-validator/validator/service"
)

var (
	passphrase = "testpass"
	hdPath     = ""
)

func FuzzRegisterValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		logger := zap.NewNop()
		// create an EOTS manager
		eotsHomeDir, eotsCfg, _, eotsStore := testutil.GenEOTSConfig(r, t)
		em, err := eotsmanager.NewLocalEOTSManager(eotsCfg, eotsStore, logger, eotsHomeDir)
		require.NoError(t, err)
		defer func() {
			err = os.RemoveAll(eotsHomeDir)
			require.NoError(t, err)
		}()

		// Create mocked babylon client
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		currentHeight := randomStartingHeight + uint64(r.Int63n(10)+2)
		mockClientController := testutil.PrepareMockedClientController(t, r, randomStartingHeight, currentHeight, &types.StakingParams{})
		mockClientController.EXPECT().QueryLatestFinalizedBlocks(gomock.Any()).Return(nil, nil).AnyTimes()

		// Create randomized config
		valHomeDir, valCfg, _, valStore := testutil.GenValConfig(r, t)
		valCfg.ValidatorModeConfig.AutoChainScanningMode = false
		valCfg.ValidatorModeConfig.StaticChainScanningStartHeight = randomStartingHeight
		app, err := service.NewValidatorApp(valCfg, mockClientController, em, logger, valStore)
		require.NoError(t, err)
		defer func() {
			err = os.RemoveAll(valHomeDir)
			require.NoError(t, err)
		}()

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
				testutil.ZeroCommissionRate(),
				testutil.EmptyDescription(),
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
