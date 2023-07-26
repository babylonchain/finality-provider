package e2etest

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	// current number of active test nodes. This is necessary to replicate btcd rpctest.Harness
	// methods of generating keys i.e with each started btcd node we increment this number
	// by 1, and then use hdSeed || numTestInstances as the seed for generating keys
	NumTestInstances = 0
	StakingTime      = 7
	StakingAmount    = 10000
)

type TestManager struct {
	BabylonHandler *BabylonNodeHandler
	Config         *valcfg.Config
	Va             *service.ValidatorApp
	BabylonClient  *babylonclient.BabylonController
}

type TestDelegationData struct {
	DelegatorPrivKey        *btcec.PrivateKey
	DelegatorKey            *btcec.PublicKey
	DelegatorBabylonPrivKey *secp256k1.PrivKey
	DelegatorBabylonKey     *secp256k1.PubKey
	JuryPrivKey             *btcec.PrivateKey
	JuryKey                 *btcec.PublicKey
	StakingTx               *bstypes.StakingTx
	SlashingTx              *bstypes.BTCSlashingTx
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *types.BIP340Signature

	SlashingAddr  string
	StakingTime   uint16
	StakingAmount int64
}

func StartManager(t *testing.T, isJury bool) *TestManager {
	bh, err := NewBabylonNodeHandler()
	require.NoError(t, err)

	err = bh.Start()
	require.NoError(t, err)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout

	// setup validator config
	testDir, err := tempDirWithName("vale2etest")
	require.NoError(t, err)

	cfg := defaultValidatorConfig(bh.GetNodeDataDir(), testDir, isJury)

	bl, err := babylonclient.NewBabylonController(cfg.BabylonConfig, logger)
	require.NoError(t, err)

	valApp, err := service.NewValidatorAppFromConfig(cfg, logger, bl)
	require.NoError(t, err)

	err = valApp.Start()
	require.NoError(t, err)

	NumTestInstances++

	return &TestManager{
		BabylonHandler: bh,
		Config:         cfg,
		Va:             valApp,
		BabylonClient:  bl,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.Va.Stop()
	require.NoError(t, err)
	err = tm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.Config.DatabaseConfig.Path)
	require.NoError(t, err)
	err = os.RemoveAll(tm.Config.BabylonConfig.KeyDirectory)
	require.NoError(t, err)
}

func (tm *TestManager) InsertBTCDelegation(t *testing.T, valBtcPk *btcec.PublicKey) *TestDelegationData {
	delegatorPrivKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	delBabylonPrivKey := secp256k1.GenPrivKey()
	delBabylonPubKey := delBabylonPrivKey.PubKey()
	pop, err := bstypes.NewPoP(delBabylonPrivKey, delegatorPrivKey)
	require.NoError(t, err)
	juryPrivKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	r := rand.New(rand.NewSource(1))
	slashingAddr, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
	require.NoError(t, err)
	stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(
		r, delegatorPrivKey, valBtcPk, juryPrivKey.PubKey(), uint16(StakingTime), int64(StakingAmount), slashingAddr)
	require.NoError(t, err)

	// generate staking tx info
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, stakingMsgTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, stakingTx.Tx, btcHeaderWithProof.SpvProof.MerkleNodes)

	// generate proper delegator sig
	delegatorSig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		delegatorPrivKey,
		&chaincfg.SimNetParams,
	)
	require.NoError(t, err)

	_, err = tm.BabylonClient.CreateBTCDelegation(
		delBabylonPubKey.(*secp256k1.PubKey), pop, stakingTx, txInfo, slashingTx, delegatorSig)
	require.NoError(t, err)

	return &TestDelegationData{
		DelegatorPrivKey:        delegatorPrivKey,
		DelegatorKey:            delegatorPrivKey.PubKey(),
		DelegatorBabylonPrivKey: delBabylonPrivKey,
		DelegatorBabylonKey:     delBabylonPubKey.(*secp256k1.PubKey),
		JuryPrivKey:             juryPrivKey,
		JuryKey:                 juryPrivKey.PubKey(),
		StakingTx:               stakingTx,
		SlashingTx:              slashingTx,
		StakingTxInfo:           txInfo,
		DelegatorSig:            delegatorSig,
		SlashingAddr:            slashingAddr,
		StakingTime:             uint16(StakingTime),
		StakingAmount:           int64(StakingAmount),
	}
}

func defaultValidatorConfig(keyringDir, testDir string, isJury bool) *valcfg.Config {
	cfg := valcfg.DefaultConfig()
	// babylon configs for sending transactions
	cfg.BabylonConfig.KeyDirectory = keyringDir
	// need to use this one to send otherwise we will have account sequence mismatch
	// errors
	cfg.BabylonConfig.Key = "test-spending-key"
	// Big adjustment to make sure we have enough gas in our transactions
	cfg.BabylonConfig.GasAdjustment = 3.0
	cfg.DatabaseConfig.Path = filepath.Join(testDir, "db")
	cfg.JuryMode = isJury

	return &cfg
}

func tempDirWithName(name string) (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, name)
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}
