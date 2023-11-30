package service_test

import (
	"encoding/hex"
	"math/rand"
	"os"
	"testing"

	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
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

func FuzzAddCovenantSig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		params := testutil.GenRandomParams(r, t)
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		finalizedHeight := randomStartingHeight + uint64(r.Int63n(10)+1)
		currentHeight := finalizedHeight + uint64(r.Int63n(10)+2)
		mockClientController := testutil.PrepareMockedClientController(t, r, finalizedHeight, currentHeight, params)

		// create a Covenant key pair in the keyring
		covenantConfig := covcfg.DefaultConfig()
		covenantSk, covenantPk, err := covenant.CreateCovenantKey(
			covenantConfig.BabylonConfig.KeyDirectory,
			covenantConfig.BabylonConfig.ChainID,
			covenantConfig.BabylonConfig.Key,
			covenantConfig.BabylonConfig.KeyringBackend,
			passphrase,
			hdPath,
		)
		require.NoError(t, err)

		// create and start covenant emulator
		ce, err := covenant.NewCovenantEmulator(&covenantConfig, mockClientController, passphrase, logrus.New())
		require.NoError(t, err)
		err = ce.Start()
		require.NoError(t, err)
		defer func() {
			err = ce.Stop()
			require.NoError(t, err)
			err := os.RemoveAll(covenantConfig.CovenantDir)
			require.NoError(t, err)
		}()

		// generate BTC delegation
		slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		changeAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		stakingTimeBlocks := uint16(5)
		stakingValue := int64(2 * 10e8)
		covThreshold := datagen.RandomInt(r, 5) + 1
		covNum := covThreshold * 2
		covenantPks := testutil.GenBtcPublicKeys(r, t, int(covNum))
		valNum := datagen.RandomInt(r, 5) + 1
		valPks := testutil.GenBtcPublicKeys(r, t, int(valNum))
		slashingRate := testutil.GenRandomDec(r)
		testInfo := datagen.GenBTCStakingSlashingInfo(
			r,
			t,
			&chaincfg.SimNetParams,
			delSK,
			valPks,
			covenantPks,
			uint32(covThreshold),
			stakingTimeBlocks,
			stakingValue,
			slashingAddr.String(),
			changeAddr.String(),
			slashingRate,
		)
		stakingTxBytes, err := bbntypes.SerializeBTCTx(testInfo.StakingTx)
		require.NoError(t, err)
		btcDel := &types.Delegation{
			BtcPk:            delPK,
			ValBtcPks:        valPks,
			StartHeight:      1000, // not relevant here
			EndHeight:        uint64(1000 + stakingTimeBlocks),
			TotalSat:         uint64(stakingValue),
			StakingTxHex:     hex.EncodeToString(stakingTxBytes),
			StakingOutputIdx: 0,
			SlashingTxHex:    testInfo.SlashingTx.ToHexStr(),
		}

		// generate covenant sigs
		slashingSpendInfo, err := testInfo.StakingInfo.SlashingPathSpendInfo()
		require.NoError(t, err)
		covSigs := make([][]byte, 0, len(valPks))
		for _, valPk := range valPks {
			encKey, err := asig.NewEncryptionKeyFromBTCPK(valPk)
			require.NoError(t, err)
			covenantSig, err := testInfo.SlashingTx.EncSign(testInfo.StakingTx, 0, slashingSpendInfo.GetPkScriptPath(), covenantSk, encKey)
			require.NoError(t, err)
			covSigs = append(covSigs, covenantSig.MustMarshal())
		}

		//
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().QueryPendingDelegations(gomock.Any()).
			Return([]*types.Delegation{btcDel}, nil).AnyTimes()
		mockClientController.EXPECT().SubmitCovenantSigs(
			covenantPk,
			testInfo.StakingTx.TxHash().String(),
			covSigs,
		).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		covenantConfig.SlashingAddress = slashingAddr.String()
	})
}
