package keyring_test

import (
	"math/rand"
	"os"
	"testing"

	valstore "github.com/babylonchain/btc-validator/validator/store"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"

	valkr "github.com/babylonchain/btc-validator/keyring"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/testutil"
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
		kc, err := valkr.NewChainKeyringController(sdkCtx, keyName, keyring.BackendTest)
		require.NoError(t, err)

		eotsHome, cfg, logger, store := testutil.GenEOTSConfig(r, t)
		em, err := eotsmanager.NewLocalEOTSManager(cfg, store, logger, eotsHome)
		defer func() {
			err := os.RemoveAll(eotsHome)
			require.NoError(t, err)
		}()
		require.NoError(t, err)

		btcPkBytes, err := em.CreateKey(keyName, passphrase, hdPath)
		require.NoError(t, err)
		btcPk, err := types.NewBIP340PubKey(btcPkBytes)
		require.NoError(t, err)
		keyPair, err := kc.CreateChainKey(passphrase, hdPath)
		require.NoError(t, err)
		bbnPk := &secp256k1.PubKey{Key: keyPair.PublicKey.SerializeCompressed()}
		valRecord, err := em.KeyRecord(btcPk.MustMarshal(), passphrase)
		require.NoError(t, err)
		pop, err := kc.CreatePop(valRecord.PrivKey, passphrase)
		require.NoError(t, err)
		validator := valstore.NewStoreValidator(bbnPk, btcPk, keyName, chainID, pop, testutil.EmptyDescription(), testutil.ZeroCommissionRate())

		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		err = pop.Verify(bbnPk, btcPk, &chaincfg.SimNetParams)
		require.NoError(t, err)
	})
}
