package store_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/finality-provider/finality-provider/config"
	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/babylonchain/finality-provider/testutil"
)

// FuzzFinalityProvidersStore tests save and list finality providers properly
func FuzzFinalityProvidersStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		homePath := t.TempDir()
		cfg := config.DefaultDBConfigWithHomePath(homePath)

		vs, err := fpstore.NewFinalityProviderStore(&cfg)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(homePath)
			require.NoError(t, err)
		}()

		fp := testutil.GenRandomFinalityProvider(r, t)
		err = vs.CreateFinalityProvider(
			fp.ChainPk,
			fp.BtcPk,
			fp.Description,
			fp.Commission,
			fp.KeyName,
			fp.ChainID,
			fp.Pop.ChainSig,
			fp.Pop.BtcSig,
		)
		require.NoError(t, err)

		fpList, err := vs.GetAllStoredFinalityProviders()
		require.NoError(t, err)
		require.True(t, fp.BtcPk.IsEqual(fpList[0].BtcPk))

		actualFp, err := vs.GetFinalityProvider(fp.BtcPk)
		require.NoError(t, err)
		require.Equal(t, fp.BtcPk, actualFp.BtcPk)
	})
}
