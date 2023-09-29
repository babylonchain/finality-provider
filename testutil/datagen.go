package testutil

import (
	"encoding/hex"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/codec"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

func GenRandomByteArray(r *rand.Rand, length uint64) []byte {
	newHeaderBytes := make([]byte, length)
	r.Read(newHeaderBytes)
	return newHeaderBytes
}

func GenRandomHexStr(r *rand.Rand, length uint64) string {
	randBytes := GenRandomByteArray(r, length)
	return hex.EncodeToString(randBytes)
}

func AddRandomSeedsToFuzzer(f *testing.F, num uint) {
	// Seed based on the current time
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var idx uint
	for idx = 0; idx < num; idx++ {
		f.Add(r.Int63())
	}
}

func GenRandomValidator(r *rand.Rand, t *testing.T) *proto.StoreValidator {
	// generate BTC key pair
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

	// generate Babylon key pair
	babylonSK, babylonPK, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)

	// generate and verify PoP, correct case
	pop, err := bstypes.NewPoP(babylonSK, btcSK)
	require.NoError(t, err)
	err = pop.Verify(babylonPK, bip340PK)
	require.NoError(t, err)

	return &proto.StoreValidator{
		KeyName:   GenRandomHexStr(r, 4),
		BabylonPk: babylonPK.Bytes(),
		BtcPk:     bip340PK.MustMarshal(),
		Pop: &proto.ProofOfPossession{
			BabylonSig: pop.BabylonSig,
			BtcSig:     pop.BtcSig,
		},
	}
}

func GenBlocks(r *rand.Rand, startHeight, endHeight uint64) []*types.BlockInfo {
	blocks := make([]*types.BlockInfo, 0)
	for i := startHeight; i <= endHeight; i++ {
		b := &types.BlockInfo{
			Height:         i,
			LastCommitHash: datagen.GenRandomLastCommitHash(r),
		}
		blocks = append(blocks, b)
	}

	return blocks
}

// GenStoredValidator generates a random validator from the keyring and store it in DB
func GenStoredValidator(r *rand.Rand, t *testing.T, app *service.ValidatorApp) *proto.StoreValidator {
	// generate keyring
	keyName := GenRandomHexStr(r, 4)
	chainID := GenRandomHexStr(r, 4)

	res, err := app.CreateValidator(keyName, chainID, "", EmptyDescription(), ZeroCommissionRate())
	require.NoError(t, err)

	storedVal, err := app.GetValidatorStore().GetStoreValidator(res.ValPk.MustMarshal())
	require.NoError(t, err)
	return storedVal
}

func GenDBConfig(r *rand.Rand, t *testing.T) *valcfg.DatabaseConfig {
	bucketName := GenRandomHexStr(r, 10) + "-bbolt.db"
	path := filepath.Join(t.TempDir(), bucketName)
	dbcfg, err := valcfg.NewDatabaseConfig(
		"bbolt",
		path,
		bucketName,
	)
	require.NoError(t, err)
	return dbcfg
}

func GenEOTSConfig(r *rand.Rand, t *testing.T) *valcfg.EOTSManagerConfig {
	bucketName := GenRandomHexStr(r, 10) + "-bbolt.db"
	path := filepath.Join(t.TempDir(), bucketName)
	eotsCfg, err := valcfg.NewEOTSManagerConfig(
		"bbolt",
		path,
		bucketName,
	)
	require.NoError(t, err)
	return eotsCfg
}

func GenSdkContext(r *rand.Rand, t *testing.T) client.Context {
	chainID := "testchain-" + GenRandomHexStr(r, 4)
	dir := t.TempDir()
	return client.Context{}.
		WithChainID(chainID).
		WithCodec(codec.MakeCodec()).
		WithKeyringDir(dir)
}
