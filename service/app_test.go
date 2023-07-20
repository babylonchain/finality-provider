package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/proto"
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

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		s := app.GetValidatorStore()
		validator := testutil.GenRandomValidator(r, t)
		err = s.SaveValidator(validator)
		require.NoError(t, err)

		// TODO avoid conversion after btcstaking protos are introduced
		// decode db object to specific types
		btcPk := validator.MustGetBIP340BTCPK()
		bbnPk := validator.GetBabylonPK()
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

		val, err := s.GetValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, val.Status, proto.ValidatorStatus_VALIDATOR_STATUS_REGISTERED)

	})
}

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.BabylonConfig.KeyDirectory = t.TempDir()
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
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

		btcPk := validator.MustGetBIP340BTCPK()
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
