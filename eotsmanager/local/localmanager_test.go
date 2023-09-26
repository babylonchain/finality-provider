package local_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager/local"
	"github.com/babylonchain/btc-validator/testutil"
)

// FuzzCreateValidator tests the creation of validator
func FuzzCreateValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		valName := testutil.GenRandomHexStr(r, 4)
		sdkCtx := testutil.GenSdkContext(r, t)
		eotsCfg := testutil.GenEOTSConfig(r, t)
		defer func() {
			err := os.RemoveAll(sdkCtx.KeyringDir)
			require.NoError(t, err)
			err = os.RemoveAll(eotsCfg.DBPath)
			require.NoError(t, err)
		}()

		lm, err := local.NewLocalEOTSManager(sdkCtx, "test", eotsCfg)
		require.NoError(t, err)

		valPk, err := lm.CreateValidator(valName, "")
		require.NoError(t, err)

		err = lm.Close()
		require.NoError(t, err)

		db, err := local.NewEOTSStore(eotsCfg)
		require.NoError(t, err)
		storedKeyName, err := db.GetValidatorKeyName(valPk.MustMarshal())
		require.NoError(t, err)
		require.Equal(t, valName, storedKeyName)
	})
}
