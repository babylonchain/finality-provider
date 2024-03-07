package store_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/eotsmanager/store"
	"github.com/babylonchain/finality-provider/testutil"
)

// FuzzEOTSStore tests save and show EOTS key names properly
func FuzzEOTSStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		homePath := t.TempDir()
		cfg := config.DefaultDBConfigWithHomePath(homePath)

		vs, err := store.NewEOTSStore(&cfg)
		require.NoError(t, err)

		defer func() {
			err := os.RemoveAll(homePath)
			require.NoError(t, err)
		}()

		expectedKeyName := testutil.GenRandomHexStr(r, 10)
		btcPk := testutil.GenRandomBtcPubKey(r, t)

		// add key name for the first time
		err = vs.AddEOTSKeyName(
			btcPk,
			expectedKeyName,
		)
		require.NoError(t, err)

		// add duplicate key name
		err = vs.AddEOTSKeyName(
			btcPk,
			expectedKeyName,
		)
		require.ErrorIs(t, err, store.ErrDuplicateEOTSKeyName)

		keyNameFromDb, err := vs.GetEOTSKeyName(schnorr.SerializePubKey(btcPk))
		require.NoError(t, err)
		require.Equal(t, expectedKeyName, keyNameFromDb)

		_, err = vs.GetEOTSKeyName(schnorr.SerializePubKey(testutil.GenRandomBtcPubKey(r, t)))
		require.ErrorIs(t, err, store.ErrEOTSKeyNameNotFound)
	})
}
