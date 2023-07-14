package val

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
)

// FuzzValidators tests save and list validators properly
func FuzzValidatorStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		dbcfg := testutil.GenDBConfig(r, t)
		vs, err := NewValidatorStore(dbcfg)
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
	})
}
