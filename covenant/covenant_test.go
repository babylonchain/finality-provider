package covenant_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/testutil"
	"github.com/babylonchain/btc-validator/types"
)

const (
	passphrase = "testpass"
	hdPath     = ""
)

func FuzzAddCovenantSig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// prepare mocked babylon client
		randomStartingHeight := uint64(r.Int63n(100) + 1)
		finalizedHeight := randomStartingHeight + uint64(r.Int63n(10)+1)
		currentHeight := finalizedHeight + uint64(r.Int63n(10)+2)
		mockClientController := testutil.PrepareMockedClientController(t, r, finalizedHeight, currentHeight)

		// create a Covenant key pair in the keyring
		slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
		require.NoError(t, err)
		covenantConfig := covcfg.DefaultConfig()
		covenantConfig.SlashingAddress = slashingAddr.String()
		covenantPk, err := covenant.CreateCovenantKey(
			covenantConfig.BabylonConfig.KeyDirectory,
			covenantConfig.BabylonConfig.ChainID,
			covenantConfig.BabylonConfig.Key,
			covenantConfig.BabylonConfig.KeyringBackend,
			passphrase,
			hdPath,
		)
		require.NoError(t, err)
		ce, err := covenant.NewCovenantEmulator(&covenantConfig, mockClientController, passphrase, logrus.New())
		require.NoError(t, err)

		btcPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)

		// generate BTC delegation
		delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		stakingTimeBlocks := uint16(5)
		stakingValue := int64(2 * 10e8)
		stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(r, &chaincfg.SimNetParams, delSK, btcPk.MustToBTCPK(), covenantPk, stakingTimeBlocks, stakingValue, slashingAddr.String())
		require.NoError(t, err)
		require.NoError(t, err)
		stakingTxHex, err := stakingTx.ToHexStr()
		require.NoError(t, err)
		delegation := &types.Delegation{
			ValBtcPk:      btcPk.MustToBTCPK(),
			BtcPk:         delPK,
			StakingTxHex:  stakingTxHex,
			SlashingTxHex: slashingTx.ToHexStr(),
		}

		stakingMsgTx, err := stakingTx.ToMsgTx()
		require.NoError(t, err)
		expectedTxHash := testutil.GenRandomHexStr(r, 32)
		mockClientController.EXPECT().QueryPendingDelegations(gomock.Any()).
			Return([]*types.Delegation{delegation}, nil).AnyTimes()
		mockClientController.EXPECT().SubmitCovenantSig(
			delegation.ValBtcPk,
			delegation.BtcPk,
			stakingMsgTx.TxHash().String(),
			gomock.Any(),
		).
			Return(&types.TxResponse{TxHash: expectedTxHash}, nil).AnyTimes()

		res, err := ce.AddCovenantSignature(delegation)
		require.NoError(t, err)
		require.Equal(t, expectedTxHash, res.TxHash)
	})
}
