package val_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/val"
)

// FuzzValidators tests save and list validators properly
func FuzzValidatorStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		dbcfg := testutil.GenDBConfig(r, t)
		vs, err := val.NewValidatorStore(dbcfg)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(dbcfg.Path)
			require.NoError(t, err)
		}()

		validator := testutil.GenRandomValidator(r, t)
		err = vs.SaveValidator(validator)
		require.NoError(t, err)

		valList, err := vs.ListValidators()
		require.NoError(t, err)
		require.Equal(t, validator.BabylonPk, valList[0].BabylonPk)

		actualVal, err := vs.GetStoreValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, validator.BabylonPk, actualVal.BabylonPk)
		require.Equal(t, validator.BtcPk, actualVal.BtcPk)
		require.Equal(t, validator.KeyName, actualVal.KeyName)
	})
}
