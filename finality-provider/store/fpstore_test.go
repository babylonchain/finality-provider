package store_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/babylonchain/finality-provider/testutil"
)

// FuzzFinalityProvidersStore tests save and list finality providers properly
func FuzzFinalityProvidersStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		dbPath := filepath.Join(t.TempDir(), "db")
		dbcfg := testutil.GenDBConfig(r, t)
		vs, err := fpstore.NewFinalityProviderStore(dbPath, dbcfg.Name, dbcfg.Backend)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(dbPath)
			require.NoError(t, err)
		}()

		fp := testutil.GenRandomFinalityProvider(r, t)
		err = vs.SaveFinalityProvider(fp)
		require.NoError(t, err)

		fpList, err := vs.ListFinalityProviders()
		require.NoError(t, err)
		require.Equal(t, fp.BtcPk, fpList[0].BtcPk)

		actualFp, err := vs.GetStoreFinalityProvider(fp.BtcPk)
		require.NoError(t, err)
		require.Equal(t, fp.BabylonPk, actualFp.BabylonPk)
		require.Equal(t, fp.BtcPk, actualFp.BtcPk)
		require.Equal(t, fp.KeyName, actualFp.KeyName)
	})
}
