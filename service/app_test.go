package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/valcfg"
)

func FuzzRegisterValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with mocked Babylon client
		cfg := valcfg.DefaultConfig()
		bucketName := testutil.GenRandomHexStr(r, 10) + "-bbolt.db"
		path := t.TempDir() + bucketName
		dbcfg, err := valcfg.NewDatabaseConfig(
			"bbolt",
			path,
			bucketName,
		)
		cfg.DatabaseConfig = dbcfg
		defer func() {
			err := os.RemoveAll(path)
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
