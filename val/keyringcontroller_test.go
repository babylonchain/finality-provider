package val_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager/local"
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

		chainID := testutil.GenRandomHexStr(r, 4)
		kc, err := val.NewChainKeyringController(sdkCtx, keyName, chainID, "test")
		require.NoError(t, err)

		cfg := testutil.GenEOTSConfig(r, t)
		em, err := local.NewLocalEOTSManager(sdkCtx, cfg, logrus.New())
		defer func() {
			err := os.RemoveAll(sdkCtx.KeyringDir)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.DBPath)
			require.NoError(t, err)
		}()

		require.NoError(t, err)
		btcPkBytes, err := em.CreateKey(keyName, "")
		require.NoError(t, err)
		btcPk, err := types.NewBIP340PubKey(btcPkBytes)
		require.NoError(t, err)
		bbnPk, err := kc.CreateChainKey()
		require.NoError(t, err)
		valRecord, err := em.KeyRecord(btcPk.MustMarshal(), "")
		require.NoError(t, err)
		pop, err := kc.CreatePop(valRecord.PrivKey)
		require.NoError(t, err)
		validator := val.NewStoreValidator(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())

		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		err = pop.Verify(bbnPk, btcPk)
		require.NoError(t, err)
	})
}
