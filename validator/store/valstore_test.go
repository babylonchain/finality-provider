package store_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	valstore "github.com/babylonchain/btc-validator/validator/store"
)

// FuzzValidators tests save and list validators properly
func FuzzValidatorStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		dbPath := filepath.Join(t.TempDir(), "db")
		dbcfg := testutil.GenDBConfig(r, t)
		vs, err := valstore.NewValidatorStore(dbPath, dbcfg.Name, dbcfg.Backend)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(dbPath)
			require.NoError(t, err)
		}()

		validator := testutil.GenRandomValidator(r, t)
		err = vs.SaveValidator(validator)
		require.NoError(t, err)

		valList, err := vs.ListValidators()
		require.NoError(t, err)
		require.Equal(t, validator.BtcPk, valList[0].BtcPk)

		actualVal, err := vs.GetStoreValidator(validator.BtcPk)
		require.NoError(t, err)
		require.Equal(t, validator.BabylonPk, actualVal.BabylonPk)
		require.Equal(t, validator.BtcPk, actualVal.BtcPk)
		require.Equal(t, validator.KeyName, actualVal.KeyName)
	})
}
