package eotsmanager_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/types"
	"github.com/babylonchain/btc-validator/testutil"
)

// FuzzCreateKey tests the creation of an EOTS key
func FuzzCreateKey(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		valName := testutil.GenRandomHexStr(r, 4)
		eotsCfg := testutil.GenEOTSConfig(r, t)
		defer func() {
			err := os.RemoveAll(eotsCfg.KeyDirectory)
			require.NoError(t, err)
			err = os.RemoveAll(eotsCfg.DatabaseConfig.Path)
			require.NoError(t, err)
		}()

		lm, err := eotsmanager.NewLocalEOTSManager(eotsCfg, logrus.New())
		require.NoError(t, err)

		valPk, err := lm.CreateKey(valName, "")
		require.NoError(t, err)

		valRecord, err := lm.KeyRecord(valPk, "")
		require.NoError(t, err)
		require.Equal(t, valName, valRecord.Name)

		sig, err := lm.SignSchnorrSig(valPk, datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)
		require.NotNil(t, sig)

		_, err = lm.CreateKey(valName, "")
		require.ErrorIs(t, err, types.ErrValidatorAlreadyExisted)
	})
}

func FuzzCreateRandomnessPairList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		valName := testutil.GenRandomHexStr(r, 4)
		eotsCfg := testutil.GenEOTSConfig(r, t)
		defer func() {
			err := os.RemoveAll(eotsCfg.KeyDirectory)
			require.NoError(t, err)
			err = os.RemoveAll(eotsCfg.DatabaseConfig.Path)
			require.NoError(t, err)
		}()

		lm, err := eotsmanager.NewLocalEOTSManager(eotsCfg, logrus.New())
		require.NoError(t, err)

		valPk, err := lm.CreateKey(valName, "")
		require.NoError(t, err)

		chainID := datagen.GenRandomByteArray(r, 10)
		startHeight := datagen.RandomInt(r, 100)
		num := r.Intn(10) + 1
		pubRandList, err := lm.CreateRandomnessPairList(valPk, chainID, startHeight, uint32(num))
		require.NoError(t, err)
		require.Len(t, pubRandList, num)

		for i := 0; i < num; i++ {
			sig, err := lm.SignEOTS(valPk, chainID, datagen.GenRandomByteArray(r, 32), startHeight+uint64(i))
			require.NoError(t, err)
			require.NotNil(t, sig)
		}
	})
}
