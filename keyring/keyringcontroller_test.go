package keyring_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"

	fpkr "github.com/babylonchain/finality-provider/keyring"

	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/testutil"
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
		kc, err := fpkr.NewChainKeyringController(sdkCtx, keyName, keyring.BackendTest)
		require.NoError(t, err)

		eotsHome := filepath.Join(t.TempDir(), "eots-home")
		cfg := testutil.GenEOTSConfig(r, t)
		em, err := eotsmanager.NewLocalEOTSManager(eotsHome, cfg, zap.NewNop())
		defer func() {
			err := os.RemoveAll(eotsHome)
			require.NoError(t, err)
		}()
		require.NoError(t, err)

		btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
		require.NoError(t, err)
		btcPk, err := types.NewBIP340PubKey(btcPkBytes)
		require.NoError(t, err)
		keyInfo, err := kc.CreateChainKey(passphrase, hdPath)
		require.NoError(t, err)
		bbnPk := &secp256k1.PubKey{Key: keyInfo.PublicKey.SerializeCompressed()}
		fpRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
		require.NoError(t, err)
		pop, err := kc.CreatePop(fpRecord.PrivKey, passphrase)
		require.NoError(t, err)
		fp := fpstore.NewStoreFinalityProvider(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())

		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(fp.Pop.BtcSig)
		require.NoError(t, err)
		err = pop.Verify(bbnPk, btcPk, &chaincfg.SimNetParams)
		require.NoError(t, err)
	})
}
