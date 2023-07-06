package testutil

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	kvstore "github.com/babylonchain/btc-validator/store"
)

func CreateStore(r *rand.Rand, t *testing.T) (kvstore.Store, string) {
	bucketName := GenRandomHexStr(r, 10) + "-bbolt.db"
	path := t.TempDir() + bucketName
	store, err := kvstore.NewBboltStore(path, bucketName)
	require.NoError(t, err)

	return store, path
}

// CleanUp cleans up (deletes) the database file that has been created during a test.
// If an error occurs the test is NOT marked as failed.
func CleanUp(store kvstore.Store, path string, t *testing.T) {
	err := store.Close()
	require.NoError(t, err)
	err = os.RemoveAll(path)
	require.NoError(t, err)
}
