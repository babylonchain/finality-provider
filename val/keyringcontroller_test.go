package val_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/types"
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
		sdkCtx := testutil.GenSdkContext(r, t)
		defer func() {
			err := os.RemoveAll(sdkCtx.KeyringDir)
			require.NoError(t, err)
		}()

		kc, err := val.NewKeyringController(sdkCtx, keyName, "test")
		require.NoError(t, err)
		require.False(t, kc.ValidatorKeyExists())

		btcPk, bbnPk, err := kc.CreateValidatorKeys()
		require.NoError(t, err)
		require.True(t, kc.ValidatorKeyExists() && kc.ValidatorKeyNameTaken())

		pop, err := kc.CreatePop()
		require.NoError(t, err)
		validator := val.NewStoreValidator(bbnPk, btcPk, kc.GetKeyName(), pop, testutil.EmptyDescription(), testutil.ZeroCommissionRates())

		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		err = pop.Verify(bbnPk, btcPk)
		require.NoError(t, err)
	})
}
