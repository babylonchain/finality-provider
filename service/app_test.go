package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	ctrl "github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"

	"github.com/babylonchain/btc-validator/types"
)

func FuzzRegisterValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
		}()
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, startingBlock.Height, startingBlock.LastCommitHash)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockClientController)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := testutil.GenStoredValidator(r, t, app)
		btcSig := new(bbntypes.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		pop := &bstypes.ProofOfPossession{
			BabylonSig: validator.Pop.BabylonSig,
			BtcSig:     btcSig,
		}

		txHash := testutil.GenRandomHexStr(r, 32)
		defaultParams := ctrl.StakingParams{}
		mockClientController.EXPECT().GetStakingParams().Return(&defaultParams, nil)
		mockClientController.EXPECT().
			RegisterValidator(
				validator.GetBabylonPK(),
				validator.MustGetBIP340BTCPK(),
				pop,
				defaultParams.MinComissionRate,
			).Return(&provider.RelayerTxResponse{TxHash: txHash}, nil).AnyTimes()

		res, _, err := app.RegisterValidator(validator.KeyName)
		require.NoError(t, err)
		require.Equal(t, txHash, res.TxHash)

		err = app.StartHandlingValidator(validator.GetBabylonPK())
		require.NoError(t, err)

		valAfterReg, err := app.GetValidatorInstance(validator.GetBabylonPK())
		require.NoError(t, err)
		require.Equal(t, valAfterReg.GetStoreValidator().Status, proto.ValidatorStatus_REGISTERED)
	})
}

func FuzzAddJurySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.BabylonConfig.KeyDirectory = t.TempDir()
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
			require.NoError(t, err)
		}()
		startingBlock := &types.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockClientController := testutil.PrepareMockedClientController(t, startingBlock.Height, startingBlock.LastCommitHash)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockClientController)
		require.NoError(t, err)

		// create a validator object and save it to db
		validator := testutil.GenStoredValidator(r, t, app)
		btcPkBIP340 := validator.MustGetBIP340BTCPK()
		btcPk := validator.MustGetBTCPK()

		// create a Jury key pair in the keyring
		juryKc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), cfg.JuryModeConfig.JuryKeyName)
		require.NoError(t, err)
		jurPk, err := juryKc.CreateJuryKey()
		require.NoError(t, err)
		require.NotNil(t, jurPk)
		cfg.JuryMode = true

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// generate BTC delegation
		slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		stakingTimeBlocks := uint16(5)
		stakingValue := int64(2 * 10e8)
		stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(r, &chaincfg.SimNetParams, delSK, btcPk, jurPk, stakingTimeBlocks, stakingValue, slashingAddr.String())
		require.NoError(t, err)
		delBabylonSK, delBabylonPK, err := datagen.GenRandomSecp256k1KeyPair(r)
		require.NoError(t, err)
		pop, err := bstypes.NewPoP(delBabylonSK, delSK)
		require.NoError(t, err)
		require.NoError(t, err)
		delegation := &bstypes.BTCDelegation{
			ValBtcPk:   btcPkBIP340,
			BtcPk:      bbntypes.NewBIP340PubKeyFromBTCPK(delPK),
			BabylonPk:  delBabylonPK.(*secp256k1.PubKey),
			Pop:        pop,
			StakingTx:  stakingTx,
			SlashingTx: slashingTx,
		}

		stakingMsgTx, err := stakingTx.ToMsgTx()
		require.NoError(t, err)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().QueryPendingBTCDelegations().
			Return([]*bstypes.BTCDelegation{delegation}, nil).AnyTimes()
		mockClientController.EXPECT().SubmitJurySig(delegation.ValBtcPk, delegation.BtcPk, stakingMsgTx.TxHash().String(), gomock.Any()).
			Return(&provider.RelayerTxResponse{TxHash: expectedTxHash}, nil).AnyTimes()
		res, err := app.AddJurySignature(delegation)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
	})
}
