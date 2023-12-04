package testutil

import (
	"encoding/hex"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/codec"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/types"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/babylonchain/btc-validator/validator/proto"
	"github.com/babylonchain/btc-validator/validator/service"
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
	err = pop.Verify(babylonPK, bip340PK, &chaincfg.SigNetParams)
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

func GenValidSlashingRate(r *rand.Rand) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)
}

func GenRandomParams(r *rand.Rand, t *testing.T) *types.StakingParams {
	covThreshold := datagen.RandomInt(r, 5) + 1
	covNum := covThreshold * 2
	covenantPks := make([]*btcec.PublicKey, 0, covNum)
	for i := 0; i < int(covNum); i++ {
		_, covPk, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		covenantPks = append(covenantPks, covPk)
	}

	slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
	require.NoError(t, err)
	return &types.StakingParams{
		ComfirmationTimeBlocks:    10,
		FinalizationTimeoutBlocks: 100,
		MinSlashingTxFeeSat:       1,
		CovenantPks:               covenantPks,
		SlashingAddress:           slashingAddr,
		CovenantQuorum:            uint32(covThreshold),
		SlashingRate:              GenValidSlashingRate(r),
	}
}

func GenBtcPublicKeys(r *rand.Rand, t *testing.T, num int) []*btcec.PublicKey {
	pks := make([]*btcec.PublicKey, 0, num)
	for i := 0; i < num; i++ {
		_, covPk, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		pks = append(pks, covPk)
	}

	return pks
}

func GenBlocks(r *rand.Rand, startHeight, endHeight uint64) []*types.BlockInfo {
	blocks := make([]*types.BlockInfo, 0)
	for i := startHeight; i <= endHeight; i++ {
		b := &types.BlockInfo{
			Height: i,
			Hash:   datagen.GenRandomAppHash(r),
		}
		blocks = append(blocks, b)
	}

	return blocks
}

// GenStoredValidator generates a random validator from the keyring and store it in DB
func GenStoredValidator(r *rand.Rand, t *testing.T, app *service.ValidatorApp, passphrase, hdPath string) *proto.StoreValidator {
	// generate keyring
	keyName := GenRandomHexStr(r, 4)
	chainID := GenRandomHexStr(r, 4)

	res, err := app.CreateValidator(keyName, chainID, passphrase, hdPath, EmptyDescription(), ZeroCommissionRate())
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

func GenEOTSConfig(r *rand.Rand, t *testing.T) *config.Config {
	bucketName := GenRandomHexStr(r, 10) + "-bbolt.db"
	dir := t.TempDir()
	path := filepath.Join(dir, bucketName)
	dbCfg, err := config.NewDatabaseConfig("bbolt", path, bucketName)
	require.NoError(t, err)
	eotsCfg := &config.Config{
		KeyDirectory:   dir,
		KeyringBackend: keyring.BackendTest,
		DatabaseConfig: dbCfg,
	}
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
