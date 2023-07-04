package val

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/valrpc"
)

// FuzzValidators tests save and list validators properly
func FuzzValidators(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		s, path := testutil.CreateStore(r, t)
		defer testutil.CleanUp(s, path, t)

		valNum := r.Intn(10) + 1
		valList := make([]*valrpc.Validator, valNum)
		for i := 0; i < valNum; i++ {
			valList[i] = testutil.GenRandomValidator(r)
			err := SaveValidator(s, valList[i])
			require.NoError(t, err)
		}

		valList2, err := ListValidators(s)
		require.NoError(t, err)
		require.Equal(t, len(valList), len(valList2))
	})

}
