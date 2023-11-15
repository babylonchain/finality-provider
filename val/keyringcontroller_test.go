package val_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/val"
)

var (
	passphrase = "testpass"
	hdPath     = ""
)

// FuzzCreatePoP tests the creation of PoP
func FuzzCreatePoP(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		keyName := testutil.GenRandomHexStr(r, 4)
		sdkCtx := testutil.GenSdkContext(r, t)

		chainID := testutil.GenRandomHexStr(r, 4)
		kc, err := val.NewChainKeyringController(sdkCtx, keyName, "test")
		require.NoError(t, err)

		cfg := testutil.GenEOTSConfig(r, t)
		em, err := eotsmanager.NewLocalEOTSManager(cfg, logrus.New())
		defer func() {
			err := os.RemoveAll(sdkCtx.KeyringDir)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.KeyringBackend)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
		}()
		require.NoError(t, err)

		btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
		require.NoError(t, err)
		btcPk, err := types.NewBIP340PubKey(btcPkBytes)
		require.NoError(t, err)
		bbnPk, err := kc.CreateChainKey(passphrase, hdPath)
		require.NoError(t, err)
		valRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
		require.NoError(t, err)
		pop, err := kc.CreatePop(valRecord.PrivKey)
		require.NoError(t, err)
		validator := val.NewStoreValidator(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())

		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		err = pop.Verify(bbnPk, btcPk, &chaincfg.SimNetParams)
		require.NoError(t, err)
	})
}
