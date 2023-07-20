package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
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
		}()
		ctl := gomock.NewController(t)
		mockBabylonClient := mocks.NewMockBabylonClient(ctl)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		// create a validator object and save it to db
		s := app.GetValidatorStore()
		validator := testutil.GenRandomValidator(r, t)
		err = s.SaveValidator(validator)
		require.NoError(t, err)

		// TODO avoid conversion after btcstaking protos are introduced
		// decode db object to specific types
		btcPk := new(types.BIP340PubKey)
		err = btcPk.Unmarshal(validator.BtcPk)
		require.NoError(t, err)
		bbnPk := &secp256k1.PubKey{Key: validator.BabylonPk}
		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		pop := &bstypes.ProofOfPossession{
			BabylonSig: validator.Pop.BabylonSig,
			BtcSig:     btcSig,
		}

		txHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			RegisterValidator(bbnPk, btcPk, pop).Return(txHash, nil).AnyTimes()

		actualTxHash, err := app.RegisterValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, txHash, actualTxHash)
	})
}

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.KeyringDir = t.TempDir()
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.KeyringDir)
			require.NoError(t, err)
		}()
		ctl := gomock.NewController(t)
		mockBabylonClient := mocks.NewMockBabylonClient(ctl)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		// create a validator object and save it to db
		keyName := testutil.GenRandomHexStr(r, 4)
		kc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), keyName)
		require.NoError(t, err)
		validator, err := kc.CreateBTCValidator()
		require.NoError(t, err)
		s := app.GetValidatorStore()
		err = s.SaveValidator(validator)
		require.NoError(t, err)

		btcPk := new(types.BIP340PubKey)
		err = btcPk.Unmarshal(validator.BtcPk)
		require.NoError(t, err)
		txHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(btcPk, uint64(1), gomock.Any(), gomock.Any()).
			Return(txHash, nil).AnyTimes()
		num := r.Intn(10) + 1
		txHashes, err := app.CommitPubRandForAll(uint64(num))
		require.NoError(t, err)
		require.Equal(t, txHash, txHashes[0])

		// check the last_committed_height
		updatedVal, err := s.GetValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, uint64(num), updatedVal.LastCommittedHeight)

		// check the committed pub rand
		for i := 1; i <= num; i++ {
			randPair, err := s.GetRandPair(validator.BabylonPk, uint64(i))
			require.NoError(t, err)
			require.NotNil(t, randPair)
		}
	})
}

func FuzzAddJurySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.KeyringDir = t.TempDir()
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.KeyringDir)
			require.NoError(t, err)
		}()
		ctl := gomock.NewController(t)
		mockBabylonClient := mocks.NewMockBabylonClient(ctl)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		// create a validator object and save it to db
		keyName := testutil.GenRandomHexStr(r, 4)
		kc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), keyName)
		require.NoError(t, err)
		validator, err := kc.CreateBTCValidator()
		require.NoError(t, err)
		s := app.GetValidatorStore()
		err = s.SaveValidator(validator)
		require.NoError(t, err)
		btcPkBIP340 := new(types.BIP340PubKey)
		err = btcPkBIP340.Unmarshal(validator.BtcPk)
		require.NoError(t, err)
		btcPk, err := btcPkBIP340.ToBTCPK()
		require.NoError(t, err)

		// create a Jury key pair in the keyring
		juryKeyName := testutil.GenRandomHexStr(r, 4)
		juryKc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), juryKeyName)
		require.NoError(t, err)
		jurPk, err := juryKc.CreateJuryKey()
		require.NoError(t, err)
		require.NotNil(t, jurPk)
		cfg.JuryKeyName = juryKeyName

		// generate BTC delegation
		slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		stakingTimeBlocks := uint16(5)
		stakingValue := int64(2 * 10e8)
		stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(r, delSK, btcPk, jurPk, stakingTimeBlocks, stakingValue, slashingAddr)
		require.NoError(t, err)
		stakingMsgTx, err := stakingTx.ToMsgTx()
		require.NoError(t, err)
		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		// random Babylon SK
		delBabylonSK, delBabylonPK, err := datagen.GenRandomSecp256k1KeyPair(r)
		require.NoError(t, err)
		pop, err := bstypes.NewPoP(delBabylonSK, delSK)
		require.NoError(t, err)
		delegatorSig, err := slashingTx.Sign(
			stakingMsgTx,
			stakingTx.StakingScript,
			delSK,
			&chaincfg.SimNetParams,
		)
		require.NoError(t, err)
		delegation := &bstypes.BTCDelegation{
			ValBtcPk:     btcPkBIP340,
			BtcPk:        types.NewBIP340PubKeyFromBTCPK(delPK),
			BabylonPk:    delBabylonPK.(*secp256k1.PubKey),
			Pop:          pop,
			StakingTx:    stakingTx,
			SlashingTx:   slashingTx,
			DelegatorSig: delegatorSig,
		}

		expectedTxHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().SubmitJurySig(delegation.ValBtcPk, delegation.BtcPk, gomock.Any()).
			Return(expectedTxHash, nil).AnyTimes()
		_, err = app.AddJurySignature(delegation)
		txHash, err := app.AddJurySignature(delegation)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, txHash)
		_, err = app.AddJurySignature(delegation)
		require.NoError(t, err)
	})
}
