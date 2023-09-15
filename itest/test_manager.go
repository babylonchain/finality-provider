package e2etest

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	eventuallyWaitTimeOut = 30 * time.Second
	eventuallyPollTime    = 500 * time.Millisecond
)

var btcNetworkParams = &chaincfg.SimNetParams

type TestManager struct {
	Wg             sync.WaitGroup
	BabylonHandler *BabylonNodeHandler
	Config         *valcfg.Config
	Va             *service.ValidatorApp
	BabylonClient  *clientcontroller.BabylonController
}

type TestDelegationData struct {
	DelegatorPrivKey        *btcec.PrivateKey
	DelegatorKey            *btcec.PublicKey
	DelegatorBabylonPrivKey *secp256k1.PrivKey
	DelegatorBabylonKey     *secp256k1.PubKey
	StakingTx               *bstypes.BabylonBTCTaprootTx
	SlashingTx              *bstypes.BTCSlashingTx
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *bbntypes.BIP340Signature

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

	bc, err := clientcontroller.NewBabylonController(bh.GetNodeDataDir(), cfg.BabylonConfig, logger)
	require.NoError(t, err)

	valApp, err := service.NewValidatorAppFromConfig(cfg, logger, bc)
	require.NoError(t, err)

	err = valApp.Start()
	require.NoError(t, err)

	tm := &TestManager{
		BabylonHandler: bh,
		Config:         cfg,
		Va:             valApp,
		BabylonClient:  bc,
	}

	tm.WaitForNodeStart(t)

	return tm
}

func (tm *TestManager) WaitForNodeStart(t *testing.T) {

	require.Eventually(t, func() bool {
		_, err := tm.BabylonClient.GetStakingParams()

		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
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

func (tm *TestManager) WaitForValRegistered(t *testing.T, bbnPk *secp256k1.PubKey) {
	require.Eventually(t, func() bool {
		queriedValidators, err := tm.BabylonClient.QueryValidators()
		if err != nil {
			return false
		}
		return len(queriedValidators) == 1 && queriedValidators[0].BabylonPk.Equals(bbnPk)
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) WaitForValPubRandCommitted(t *testing.T, valIns *service.ValidatorInstance) {
	require.Eventually(t, func() bool {
		randPairs, err := valIns.GetCommittedPubRandPairList()
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randPairs)
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) WaitForNPendingDels(t *testing.T, n int) []*bstypes.BTCDelegation {
	var (
		dels []*bstypes.BTCDelegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
		if err != nil {
			return false
		}
		return len(dels) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return dels
}

func (tm *TestManager) WaitForValNActiveDels(t *testing.T, btcPk *bbntypes.BIP340PubKey, n int) []*bstypes.BTCDelegation {
	var dels []*bstypes.BTCDelegation
	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(btcPk, 1000)
		if err != nil {
			return false
		}
		return len(dels) == n && CheckDelsStatus(dels, currentBtcTip.Height, params.FinalizationTimeoutBlocks, bstypes.BTCDelegationStatus_ACTIVE)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return dels
}

func (tm *TestManager) WaitForValNUnbondingDels(t *testing.T, btcPk *bbntypes.BIP340PubKey, n int) []*bstypes.BTCDelegation {
	var (
		dels []*bstypes.BTCDelegation
		err  error
	)
	// wait for our validator to:
	// - detect new unbonding
	// - send signature
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(btcPk, 1000)
		if err != nil {
			return false
		}

		return len(dels) == 1 && dels[0].BtcUndelegation != nil && dels[0].BtcUndelegation.ValidatorUnbondingSig != nil

	}, 1*time.Minute, eventuallyPollTime)

	return dels
}

func CheckDelsStatus(dels []*bstypes.BTCDelegation, btcHeight uint64, w uint64, status bstypes.BTCDelegationStatus) bool {
	allChecked := true
	for _, d := range dels {
		s := d.GetStatus(btcHeight, w)
		if s != status {
			allChecked = false
		}
	}

	return allChecked
}

func (tm *TestManager) WaitForNFinalizedBlocks(t *testing.T, n int) []*types.BlockInfo {
	var (
		blocks []*types.BlockInfo
		err    error
	)
	require.Eventually(t, func() bool {
		blocks, err = tm.BabylonClient.QueryLatestFinalizedBlocks(100)
		if err != nil {
			return false
		}
		return len(blocks) >= n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return blocks
}

func (tm *TestManager) StopAndRestartValidatorAfterNBlocks(t *testing.T, n int, valIns *service.ValidatorInstance) {
	headerBeforeStop, err := tm.BabylonClient.QueryBestHeader()
	require.NoError(t, err)
	err = valIns.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		headerAfterStop, err := tm.BabylonClient.QueryBestHeader()
		if err != nil {
			return false
		}

		return headerAfterStop.Header.Height >= int64(n)+headerBeforeStop.Header.Height
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Log("restarting the validator instance")

	tm.Config.ValidatorModeConfig.AutoChainScanningMode = true
	err = valIns.Start()
	require.NoError(t, err)
}

func (tm *TestManager) AddJurySignature(t *testing.T, btcDel *bstypes.BTCDelegation) *provider.RelayerTxResponse {
	slashingTx := btcDel.SlashingTx
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)

	// get Jury private key from the keyring
	juryPrivKey := tm.GetJuryPrivKey(t)

	jurySig, err := slashingTx.Sign(
		stakingMsgTx,
		stakingTx.Script,
		juryPrivKey,
		&tm.Config.ActiveNetParams,
	)
	require.NoError(t, err)

	res, err := tm.BabylonClient.SubmitJurySig(btcDel.ValBtcPk, btcDel.BtcPk, stakingMsgTx.TxHash().String(), jurySig)
	require.NoError(t, err)

	return res
}

// delegation must containt undelgation object
func (tm *TestManager) AddValidatorUnbondingSignature(
	t *testing.T,
	btcDel *bstypes.BTCDelegation,
	validatorSk *btcec.PrivateKey,
) {
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)

	unbondingTx := btcDel.BtcUndelegation.UnbondingTx

	valSig, err := unbondingTx.Sign(
		stakingMsgTx,
		stakingTx.Script,
		validatorSk,
		btcNetworkParams,
	)
	require.NoError(t, err)

	stakingTxHash := stakingMsgTx.TxHash().String()

	_, err = tm.BabylonClient.SubmitValidatorUnbondingSig(
		btcDel.ValBtcPk,
		btcDel.BtcPk,
		stakingTxHash,
		valSig,
	)
	require.NoError(t, err)
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
		r, btcNetworkParams, delBtcPrivKey, valBtcPk, juryPk, stakingTime, stakingAmount, tm.BabylonHandler.GetSlashingAddress())
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
	headers := make([]*bbntypes.BTCHeaderBytes, 0)
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
		stakingTx.Script,
		delBtcPrivKey,
		btcNetworkParams,
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

func (tm *TestManager) InsertBTCUnbonding(
	t *testing.T,
	stakingTx *bstypes.BabylonBTCTaprootTx,
	stakerPrivKey *btcec.PrivateKey,
	validatorPk *btcec.PublicKey,
) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	slashingAddr := params.SlashingAddress
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)
	stakingTxHash := stakingTx.MustGetTxHash()
	stakingTxChainHash, err := chainhash.NewHashFromStr(stakingTxHash)
	require.NoError(t, err)

	stakingOutputIdx, err := btcstaking.GetIdxOutputCommitingToScript(
		stakingMsgTx, stakingTx.Script, btcNetworkParams,
	)
	require.NoError(t, err)

	stakingValue := stakingMsgTx.TxOut[stakingOutputIdx].Value
	fee := int64(1000)

	unbondingTx, slashUnbondingTx, err := datagen.GenBTCUnbondingSlashingTx(
		r,
		btcNetworkParams,
		stakerPrivKey,
		validatorPk,
		params.JuryPk,
		wire.NewOutPoint(stakingTxChainHash, uint32(stakingOutputIdx)),
		uint16(params.FinalizationTimeoutBlocks)+1,
		stakingValue-fee,
		slashingAddr,
	)
	require.NoError(t, err)

	unbondingTxMsg, err := unbondingTx.ToMsgTx()
	require.NoError(t, err)

	slashingTxSig, err := slashUnbondingTx.Sign(
		unbondingTxMsg,
		unbondingTx.Script,
		stakerPrivKey,
		btcNetworkParams,
	)
	require.NoError(t, err)

	_, err = tm.BabylonClient.CreateBTCUndelegation(
		unbondingTx, slashUnbondingTx, slashingTxSig,
	)
	require.NoError(t, err)
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
	cfg.JuryModeConfig.QueryInterval = 7 * time.Second
	cfg.UnbondingSigSubmissionInterval = 3 * time.Second

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
