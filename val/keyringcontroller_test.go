package val_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/val"
)

// FuzzCreatePoP tests the creation of PoP
func FuzzCreatePoP(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		keyName := testutil.GenRandomHexStr(r, 4)
		// create keyring
		kr, dir := testutil.GenKeyring(r, t)
		defer func() {
			err := os.RemoveAll(dir)
			require.NoError(t, err)
		}()

		kc := val.NewKeyringController(keyName, kr)
		require.False(t, kc.KeyExists())

		btcPk, err := kc.CreateBIP340PubKey()
		require.NoError(t, err)

		bbnPk, err := kc.CreateBabylonKey()
		require.NoError(t, err)

		pop, err := kc.CreatePop()
		require.NoError(t, err)

		err = pop.Verify(bbnPk, btcPk)
		require.NoError(t, err)
	})
}
