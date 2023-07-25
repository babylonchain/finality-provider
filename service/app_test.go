package service_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/testutil/mocks"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

func FuzzRegisterValidator(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		cfg.PollerConfig.StartingHeight = randomStartingHeight
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
		}()
		startingBlock := &service.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := prepareMockedBabylonClient(t, startingBlock)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := createValidator(r, t, app)

		// TODO avoid conversion after btcstaking protos are introduced
		// decode db object to specific types
		btcPk := validator.MustGetBIP340BTCPK()
		bbnPk := validator.GetBabylonPK()
		btcSig := new(types.BIP340Signature)
		err = btcSig.Unmarshal(validator.Pop.BtcSig)
		require.NoError(t, err)
		pop := &bstypes.ProofOfPossession{
			BabylonSig: validator.Pop.BabylonSig,
			BtcSig:     btcSig,
		}

		txHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			RegisterValidator(bbnPk, btcPk, pop).Return(txHash, nil).AnyTimes()

		actualTxHash, err := app.RegisterValidator(validator.KeyName)
		require.NoError(t, err)
		require.Equal(t, txHash, actualTxHash)

		valAfterReg, err := app.GetValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, valAfterReg.Status, proto.ValidatorStatus_REGISTERED)
	})
}

func FuzzCommitPubRandList(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.BabylonConfig.KeyDirectory = t.TempDir()
		cfg.NumPubRand = uint64(r.Intn(10) + 1)
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		cfg.PollerConfig.StartingHeight = randomStartingHeight
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
			require.NoError(t, err)
		}()
		startingBlock := &service.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := prepareMockedBabylonClient(t, startingBlock)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := createValidator(r, t, app)
		err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
		require.NoError(t, err)

		btc340Pk := validator.MustGetBIP340BTCPK()
		txHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(btc340Pk, startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(txHash, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(validator.MustGetBIP340BTCPK()).
			Return(uint64(0), nil).AnyTimes()
		txHashes, err := app.CommitPubRandForAll(startingBlock)
		require.NoError(t, err)
		require.Equal(t, txHash, txHashes[0])

		// check the last_committed_height
		updatedVal, err := app.GetValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, startingBlock.Height+cfg.NumPubRand, updatedVal.LastCommittedHeight)

		// check the committed pub rand
		randPairs, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, int(cfg.NumPubRand), len(randPairs))
	})
}

func FuzzAddJurySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.BabylonConfig.KeyDirectory = t.TempDir()
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		cfg.PollerConfig.StartingHeight = randomStartingHeight
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
			require.NoError(t, err)
		}()
		startingBlock := &service.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := prepareMockedBabylonClient(t, startingBlock)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := createValidator(r, t, app)
		btcPkBIP340 := validator.MustGetBIP340BTCPK()
		btcPk := validator.MustGetBTCPK()

		// create a Jury key pair in the keyring
		juryKc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), cfg.JuryModeConfig.JuryKeyName)
		require.NoError(t, err)
		jurPk, err := juryKc.CreateJuryKey()
		require.NoError(t, err)
		require.NotNil(t, jurPk)
		cfg.JuryMode = true

		// generate BTC delegation
		slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		stakingTimeBlocks := uint16(5)
		stakingValue := int64(2 * 10e8)
		stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(r, delSK, btcPk, jurPk, stakingTimeBlocks, stakingValue, slashingAddr)
		require.NoError(t, err)
		delBabylonSK, delBabylonPK, err := datagen.GenRandomSecp256k1KeyPair(r)
		require.NoError(t, err)
		pop, err := bstypes.NewPoP(delBabylonSK, delSK)
		require.NoError(t, err)
		require.NoError(t, err)
		delegation := &bstypes.BTCDelegation{
			ValBtcPk:   btcPkBIP340,
			BtcPk:      types.NewBIP340PubKeyFromBTCPK(delPK),
			BabylonPk:  delBabylonPK.(*secp256k1.PubKey),
			Pop:        pop,
			StakingTx:  stakingTx,
			SlashingTx: slashingTx,
		}

		expectedTxHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().QueryPendingBTCDelegations().
			Return([]*bstypes.BTCDelegation{delegation}, nil).AnyTimes()
		mockBabylonClient.EXPECT().SubmitJurySig(delegation.ValBtcPk, delegation.BtcPk, gomock.Any()).
			Return(expectedTxHash, nil).AnyTimes()
		txHash, err := app.AddJurySignature(delegation)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, txHash)
	})
}

func FuzzSubmitFinalitySig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create validator app with db and mocked Babylon client
		cfg := valcfg.DefaultConfig()
		cfg.DatabaseConfig = testutil.GenDBConfig(r, t)
		cfg.BabylonConfig.KeyDirectory = t.TempDir()
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		cfg.PollerConfig.StartingHeight = randomStartingHeight
		defer func() {
			err := os.RemoveAll(cfg.DatabaseConfig.Path)
			require.NoError(t, err)
			err = os.RemoveAll(cfg.BabylonConfig.KeyDirectory)
			require.NoError(t, err)
		}()
		startingBlock := &service.BlockInfo{Height: randomStartingHeight, LastCommitHash: testutil.GenRandomByteArray(r, 32)}
		mockBabylonClient := prepareMockedBabylonClient(t, startingBlock)
		app, err := service.NewValidatorAppFromConfig(&cfg, logrus.New(), mockBabylonClient)
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		defer func() {
			err = app.Stop()
			require.NoError(t, err)
		}()

		// create a validator object and save it to db
		validator := createValidator(r, t, app)
		err = app.GetValidatorStore().SetValidatorStatus(validator, proto.ValidatorStatus_REGISTERED)
		require.NoError(t, err)
		btcPkBIP340 := validator.MustGetBIP340BTCPK()

		// commit public randomness
		txHash := testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			CommitPubRandList(btcPkBIP340, startingBlock.Height+1, gomock.Any(), gomock.Any()).
			Return(txHash, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryHeightWithLastPubRand(btcPkBIP340).
			Return(uint64(0), nil).AnyTimes()
		txHashes, err := app.CommitPubRandForAll(startingBlock)
		require.NoError(t, err)
		require.Equal(t, txHash, txHashes[0])

		// check the committed pub rand
		randPairs, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, int(cfg.NumPubRand), len(randPairs))

		// submit finality sig
		nextBlock := &service.BlockInfo{
			Height:         startingBlock.Height + 1,
			LastCommitHash: testutil.GenRandomByteArray(r, 32),
		}
		txHash = testutil.GenRandomByteArray(r, 32)
		mockBabylonClient.EXPECT().
			SubmitFinalitySig(btcPkBIP340, nextBlock.Height, nextBlock.LastCommitHash, gomock.Any()).
			Return(txHash, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryValidatorVotingPower(btcPkBIP340, nextBlock.Height).
			Return(uint64(1), nil).AnyTimes()
		txHashes, err = app.SubmitFinalitySignaturesForAll(nextBlock)
		require.NoError(t, err)
		require.Equal(t, txHash, txHashes[0])

		// check the last_voted_height
		updatedVal, err := app.GetValidator(validator.BabylonPk)
		require.NoError(t, err)
		require.Equal(t, nextBlock.Height, updatedVal.LastVotedHeight)
	})
}

func prepareMockedBabylonClient(t *testing.T, startingBlock *service.BlockInfo) *mocks.MockBabylonClient {
	ctl := gomock.NewController(t)
	mockBabylonClient := mocks.NewMockBabylonClient(ctl)
	status := &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(startingBlock.Height + 1)},
	}
	resHeader := &coretypes.ResultHeader{
		Header: &cometbfttypes.Header{
			Height:         int64(startingBlock.Height),
			LastCommitHash: startingBlock.LastCommitHash,
		},
	}
	mockBabylonClient.EXPECT().QueryNodeStatus().Return(status, nil).AnyTimes()
	mockBabylonClient.EXPECT().QueryHeader(int64(startingBlock.Height)).Return(resHeader, nil).AnyTimes()

	return mockBabylonClient
}

// create a random validator object and save it to db
func createValidator(r *rand.Rand, t *testing.T, app *service.ValidatorApp) *proto.Validator {
	// generate keyring
	keyName := testutil.GenRandomHexStr(r, 4)
	kc, err := val.NewKeyringControllerWithKeyring(app.GetKeyring(), keyName)
	require.NoError(t, err)

	// create validator using the keyring
	validator, err := kc.CreateBTCValidator()
	require.NoError(t, err)

	// save the validator
	s := app.GetValidatorStore()
	err = s.SaveValidator(validator)
	require.NoError(t, err)

	return validator
}
