package eotsmanager_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/eotsmanager"
	eotscfg "github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/eotsmanager/types"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	passphrase = "testpass"
	hdPath     = ""
)

// FuzzCreateKey tests the creation of an EOTS key
func FuzzCreateKey(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		fpName := testutil.GenRandomHexStr(r, 4)
		homeDir := filepath.Join(t.TempDir(), "eots-home")
		eotsCfg := eotscfg.DefaultConfigWithHomePath(homeDir)
		dbBackend, err := eotsCfg.DatabaseConfig.GetDbBackend()
		require.NoError(t, err)
		defer func() {
			dbBackend.Close()
			err := os.RemoveAll(homeDir)
			require.NoError(t, err)
		}()

		lm, err := eotsmanager.NewLocalEOTSManager(homeDir, eotsCfg, dbBackend, zap.NewNop())
		require.NoError(t, err)

		fpPk, err := lm.CreateKey(fpName, passphrase, hdPath)
		require.NoError(t, err)

		fpRecord, err := lm.KeyRecord(fpPk, passphrase)
		require.NoError(t, err)
		require.Equal(t, fpName, fpRecord.Name)

		sig, err := lm.SignSchnorrSig(fpPk, datagen.GenRandomByteArray(r, 32), passphrase)
		require.NoError(t, err)
		require.NotNil(t, sig)

		_, err = lm.CreateKey(fpName, passphrase, hdPath)
		require.ErrorIs(t, err, types.ErrFinalityProviderAlreadyExisted)
	})
}

func FuzzCreateMasterRandPair(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		fpName := testutil.GenRandomHexStr(r, 4)
		homeDir := filepath.Join(t.TempDir(), "eots-home")
		eotsCfg := eotscfg.DefaultConfigWithHomePath(homeDir)
		dbBackend, err := eotsCfg.DatabaseConfig.GetDbBackend()
		defer func() {
			dbBackend.Close()
			err := os.RemoveAll(homeDir)
			require.NoError(t, err)
		}()
		require.NoError(t, err)
		lm, err := eotsmanager.NewLocalEOTSManager(homeDir, eotsCfg, dbBackend, zap.NewNop())
		require.NoError(t, err)

		fpPk, err := lm.CreateKey(fpName, passphrase, hdPath)
		require.NoError(t, err)
		fpBTCPK, err := bbn.NewBIP340PubKey(fpPk)
		require.NoError(t, err)

		chainID := datagen.GenRandomByteArray(r, 10)

		mprStr, err := lm.CreateMasterRandPair(fpPk, chainID, passphrase)
		require.NoError(t, err)
		mpr, err := eots.NewMasterPublicRandFromBase58(mprStr)
		require.NoError(t, err)

		startHeight := datagen.RandomInt(r, 100)
		num := r.Intn(10) + 1

		for i := 0; i < num; i++ {
			height := startHeight + uint64(i)
			msg := datagen.GenRandomByteArray(r, 32)

			// sign EOTS signature at each height
			sig, err := lm.SignEOTS(fpPk, chainID, msg, height, passphrase)
			require.NoError(t, err)
			require.NotNil(t, sig)

			// verify using the master public randomness and height
			pr, err := mpr.DerivePubRand(uint32(height))
			require.NoError(t, err)
			eots.Verify(fpBTCPK.MustToBTCPK(), pr, msg, sig)
		}
	})
}
