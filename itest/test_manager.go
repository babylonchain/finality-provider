package e2etest

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
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
	StakingTx               *bstypes.StakingTx
	SlashingTx              *bstypes.BTCSlashingTx
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *types.BIP340Signature

	SlashingAddr  string
	StakingTime   uint16
	StakingAmount int64
}

func StartManager(t *testing.T, isJury bool) *TestManager {
	bh := NewBabylonNodeHandler(t)

	err := bh.Start()
	require.NoError(t, err)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout

	// setup validator config
	testDir, err := tempDirWithName("vale2etest")
	require.NoError(t, err)

	cfg := defaultValidatorConfig(bh.GetNodeDataDir(), testDir, isJury)

	bl, err := babylonclient.NewBabylonController(bh.GetNodeDataDir(), cfg.BabylonConfig, logger)
	require.NoError(t, err)

	valApp, err := service.NewValidatorAppFromConfig(cfg, logger, bl)
	require.NoError(t, err)

	err = valApp.Start()
	require.NoError(t, err)

	return &TestManager{
		BabylonHandler: bh,
		Config:         cfg,
		Va:             valApp,
		BabylonClient:  bl,
	}
}

func StartManagerWithValidator(t *testing.T, n int, isJury bool) *TestManager {
	tm := StartManager(t, isJury)
	app := tm.Va

	var newValName = "test-val-"
	for i := 0; i < n; i++ {
		newValName += strconv.Itoa(i)
		_, err := app.CreateValidator(newValName)
		require.NoError(t, err)
		_, bbnPk, err := app.RegisterValidator(newValName)
		require.NoError(t, err)
		err = app.StartHandlingValidator(bbnPk)
		require.NoError(t, err)
	}

	require.Equal(t, n, len(app.ListValidatorInstances()))

	return tm
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.Va.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.Config.DatabaseConfig.Path)
	require.NoError(t, err)
	err = os.RemoveAll(tm.Config.BabylonConfig.KeyDirectory)
	require.NoError(t, err)
	err = tm.BabylonHandler.Stop()
	require.NoError(t, err)
}

func (tm *TestManager) AddJurySignature(t *testing.T, btcDel *bstypes.BTCDelegation) string {
	slashingTx := btcDel.SlashingTx
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)

	// get Jury private key from the keyring
	juryPrivKey := tm.GetJuryPrivKey(t)

	jurySig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		juryPrivKey,
		&tm.Config.JuryModeConfig.ActiveNetParams,
	)
	require.NoError(t, err)

	txHash, err := tm.BabylonClient.SubmitJurySig(btcDel.ValBtcPk, btcDel.BtcPk, stakingMsgTx.TxHash().String(), jurySig)
	require.NoError(t, err)

	return txHash
}

func (tm *TestManager) GetJuryPrivKey(t *testing.T) *btcec.PrivateKey {
	kr := tm.Va.GetKeyring()
	juryKeyName := tm.BabylonHandler.GetJuryKeyName()
	k, err := kr.Key(juryKeyName)
	require.NoError(t, err)
	localKey := k.GetLocal().PrivKey.GetCachedValue()
	require.IsType(t, &secp256k1.PrivKey{}, localKey)
	juryPrivKey, _ := btcec.PrivKeyFromBytes(localKey.(*secp256k1.PrivKey).Key)
	return juryPrivKey
}

func (tm *TestManager) InsertBTCDelegation(t *testing.T, valBtcPk *btcec.PublicKey, stakingTime uint16, stakingAmount int64) *TestDelegationData {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// examine staking params
	params, err := tm.BabylonClient.GetStakingParams()
	slashingAddr := params.SlashingAddress
	require.NoError(t, err)
	require.Equal(t, tm.BabylonHandler.GetSlashingAddress(), slashingAddr)
	require.Greater(t, stakingTime, uint16(params.ComfirmationTimeBlocks))
	juryPk, err := tm.Va.GetJuryPk()
	require.NoError(t, err)
	require.Equal(t, params.JuryPk.SerializeCompressed()[1:], juryPk.SerializeCompressed()[1:])

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTx, slashingTx, err := datagen.GenBTCStakingSlashingTx(
		r, delBtcPrivKey, valBtcPk, juryPk, stakingTime, stakingAmount, tm.BabylonHandler.GetSlashingAddress())
	require.NoError(t, err)

	// get msgTx
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)

	// delegator Babylon key pairs
	delBabylonPrivKey, delBabylonPubKey, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)

	// proof-of-possession
	pop, err := bstypes.NewPoP(delBabylonPrivKey, delBtcPrivKey)
	require.NoError(t, err)

	// create and insert BTC headers which include the staking tx to get staking tx info
	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, *currentBtcTip.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: currentBtcTip.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]*types.BTCHeaderBytes, 0)
	headers = append(headers, &blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = tm.BabylonClient.InsertBtcBlockHeaders(headers)
	require.NoError(t, err)
	btcHeader := blockWithStakingTx.HeaderBytes
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, stakingTx.Tx, blockWithStakingTx.SpvProof.MerkleNodes)

	// delegator sig
	delegatorSig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.StakingScript,
		delBtcPrivKey,
		&chaincfg.SimNetParams,
	)
	require.NoError(t, err)

	// submit the BTC delegation to Babylon
	_, err = tm.BabylonClient.CreateBTCDelegation(
		delBabylonPubKey.(*secp256k1.PubKey), pop, stakingTx, txInfo, slashingTx, delegatorSig)
	require.NoError(t, err)

	return &TestDelegationData{
		DelegatorPrivKey:        delBtcPrivKey,
		DelegatorKey:            delBtcPubKey,
		DelegatorBabylonPrivKey: delBabylonPrivKey.(*secp256k1.PrivKey),
		DelegatorBabylonKey:     delBabylonPubKey.(*secp256k1.PubKey),
		StakingTx:               stakingTx,
		SlashingTx:              slashingTx,
		StakingTxInfo:           txInfo,
		DelegatorSig:            delegatorSig,
		SlashingAddr:            tm.BabylonHandler.GetSlashingAddress(),
		StakingTime:             stakingTime,
		StakingAmount:           stakingAmount,
	}
}

func defaultValidatorConfig(keyringDir, testDir string, isJury bool) *valcfg.Config {
	cfg := valcfg.DefaultConfig()

	cfg.ValidatorModeConfig.AutoChainScanningMode = false
	// babylon configs for sending transactions
	cfg.BabylonConfig.KeyDirectory = keyringDir
	// need to use this one to send otherwise we will have account sequence mismatch
	// errors
	cfg.BabylonConfig.Key = "test-spending-key"
	// Big adjustment to make sure we have enough gas in our transactions
	cfg.BabylonConfig.GasAdjustment = 5
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
