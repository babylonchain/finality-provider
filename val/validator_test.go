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
func FuzzValidators(f *testing.F) {
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

		defer removeDbFile(path, t)

		createValRes, err := CreateValidator(NewCreateValidatorRequest(dbcfg))
		require.NoError(t, err)

		queryRes, err := QueryValidatorList(NewQueryValidatorListRequest(dbcfg))
		require.NoError(t, err)
		require.Equal(t, createValRes.BabylonPk, queryRes.Validators[0].BabylonPk)
	})
}

func removeDbFile(path string, t *testing.T) {
	err := os.RemoveAll(path)
	require.NoError(t, err)
}
