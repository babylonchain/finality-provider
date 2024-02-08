package store_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	kvstore "github.com/babylonchain/finality-provider/store"
	"github.com/babylonchain/finality-provider/testutil"
)

// FuzzBboltStore tests store interfaces works properly.
func FuzzBboltStore(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		store, path := testutil.CreateStore(r, t)
		defer testutil.CleanUp(store, path, t)

		kvNum := r.Intn(10) + 1
		kvList := genRandomKVList(kvNum, r)
		randIndex := r.Intn(kvNum)

		// Initially the key shouldn't exist
		v, err := store.Get(kvList[randIndex].Key)
		require.Error(t, err)
		require.Nil(t, v)

		// Deleting a non-existing key-value pair should NOT lead to an error
		err = store.Delete(kvList[randIndex].Key)
		require.NoError(t, err)

		// Save all the KV pairs
		for i := 0; i < kvNum; i++ {
			err = store.Put(kvList[i].Key, kvList[i].Value)
			require.NoError(t, err)
			// Storing it again should not lead to an error but just overwrite it
			err = store.Put(kvList[i].Key, kvList[i].Value)
			require.NoError(t, err)
			// Retrieve the object
			expectedBytes := kvList[i].Value
			v, err = store.Get(kvList[i].Key)
			require.NoError(t, err)
			require.Equal(t, expectedBytes, v)
			// Exists
			exists, err := store.Exists(kvList[i].Key)
			require.NoError(t, err)
			require.True(t, exists)
		}

		// List all the KV pairs
		newKvList, err := store.List(nil)
		require.NoError(t, err)
		require.Equal(t, kvNum, len(newKvList))
		require.Equal(t, len(kvList), len(newKvList))

		// Delete
		err = store.Delete(kvList[randIndex].Key)
		require.NoError(t, err)
		// Key-value pair shouldn't exist anymore
		v, err = store.Get(kvList[randIndex].Key)
		require.Error(t, err)
		require.Nil(t, v)
		exists, err := store.Exists(kvList[randIndex].Key)
		require.NoError(t, err)
		require.False(t, exists)
	})

}

func genRandomKVList(num int, r *rand.Rand) []*kvstore.KVPair {
	kvList := make([]*kvstore.KVPair, num)

	for i := 0; i < num; i++ {
		kvp := genRandomKV(r)
		kvList[i] = kvp
	}

	return kvList
}

func genRandomKV(r *rand.Rand) *kvstore.KVPair {
	k := testutil.GenRandomByteArray(r, 100)
	v := testutil.GenRandomByteArray(r, 1000)
	return &kvstore.KVPair{
		Key:   k,
		Value: v,
	}
}
