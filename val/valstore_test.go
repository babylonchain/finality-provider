package val

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/valcfg"
)

// FuzzValidators tests save and list validators properly
func FuzzValidatorStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		bucketName := testutil.GenRandomHexStr(r, 10) + "-bbolt.db"
		path := t.TempDir() + bucketName
		dbcfg, err := valcfg.NewDatabaseConfig(
			"bbolt",
			path,
			bucketName,
		)
		require.NoError(t, err)

		vs, err := NewValidatorStore(dbcfg)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(path)
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
