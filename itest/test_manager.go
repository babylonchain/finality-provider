package e2etest

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/eotsmanager/client"
	eotsconfig "github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/types"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/babylonchain/btc-validator/validator/service"
)

var (
	eventuallyWaitTimeOut = 1 * time.Minute
	eventuallyPollTime    = 500 * time.Millisecond
	btcNetworkParams      = &chaincfg.SimNetParams

	valNamePrefix   = "test-val-"
	monikerPrefix   = "moniker-"
	covenantKeyName = "covenant-key"
	chainID         = "chain-test"
	passphrase      = "testpass"
	hdPath          = ""
)

type TestManager struct {
	Wg                sync.WaitGroup
	BabylonHandler    *BabylonNodeHandler
	EOTSServerHandler *EOTSServerHandler
	CovenantEmulator  *covenant.CovenantEmulator
	ValConfig         *valcfg.Config
	EOTSConfig        *eotsconfig.Config
	CovenanConfig     *covcfg.Config
	Va                *service.ValidatorApp
	EOTSClient        *client.EOTSManagerGRpcClient
	BabylonClient     *clientcontroller.BabylonController
}

type TestDelegationData struct {
	DelegatorPrivKey        *btcec.PrivateKey
	DelegatorKey            *btcec.PublicKey
	DelegatorBabylonPrivKey *secp256k1.PrivKey
	DelegatorBabylonKey     *secp256k1.PubKey
	SlashingTx              *bstypes.BTCSlashingTx
	StakingTx               *wire.MsgTx
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *bbntypes.BIP340Signature
	ValidatorPks            []*btcec.PublicKey

	SlashingAddr  string
	ChangeAddr    string
	StakingTime   uint16
	StakingAmount int64
}

func StartManager(t *testing.T) *TestManager {
	testDir, err := tempDirWithName("vale2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. prepare covenant key, which will be used as input of Babylon node
	covenantConfig := defaultCovenantConfig(testDir)
	err = covenantConfig.Validate()
	require.NoError(t, err)
	covKeyPair, err := covenant.CreateCovenantKey(testDir, chainID, covenantKeyName, keyring.BackendTest, passphrase, hdPath)
	require.NoError(t, err)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, bbntypes.NewBIP340PubKeyFromBTCPK(covKeyPair.PublicKey))
	err = bh.Start()
	require.NoError(t, err)
	cfg := defaultValidatorConfig(bh.GetNodeDataDir(), testDir)
	bc, err := clientcontroller.NewBabylonController(cfg.BabylonConfig, &cfg.ActiveNetParams, logger)
	require.NoError(t, err)

	// 3. prepare EOTS manager
	eotsCfg := defaultEOTSConfig(t)
	eh := NewEOTSServerHandler(t, eotsCfg)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 4. prepare validator
	valApp, err := service.NewValidatorApp(cfg, bc, eotsCli, logger)
	require.NoError(t, err)
	err = valApp.Start()
	require.NoError(t, err)

	// 5. prepare covenant emulator
	ce, err := covenant.NewCovenantEmulator(covenantConfig, bc, passphrase, logger)
	require.NoError(t, err)
	err = ce.Start()
	require.NoError(t, err)

	tm := &TestManager{
		BabylonHandler:    bh,
		EOTSServerHandler: eh,
		ValConfig:         cfg,
		EOTSConfig:        eotsCfg,
		Va:                valApp,
		CovenantEmulator:  ce,
		CovenanConfig:     covenantConfig,
		EOTSClient:        eotsCli,
		BabylonClient:     bc,
	}

	tm.WaitForServicesStart(t)

	return tm
}

func (tm *TestManager) WaitForServicesStart(t *testing.T) {
	// wait for Babylon node starts
	require.Eventually(t, func() bool {
		_, err := tm.BabylonClient.QueryStakingParams()

		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("Babylon node is started")
}

func StartManagerWithValidator(t *testing.T) (*TestManager, *service.ValidatorInstance) {
	tm := StartManager(t)
	app := tm.Va

	valName := valNamePrefix + strconv.Itoa(0)
	moniker := monikerPrefix + strconv.Itoa(0)
	commission := sdkmath.LegacyZeroDec()
	desc, err := newDescription(moniker).Marshal()
	require.NoError(t, err)
	res, err := app.CreateValidator(valName, chainID, passphrase, hdPath, desc, &commission)
	require.NoError(t, err)
	_, err = app.RegisterValidator(res.ValPk.MarshalHex())
	require.NoError(t, err)
	err = app.StartHandlingValidator(res.ValPk, passphrase)
	require.NoError(t, err)
	valIns, err := app.GetValidatorInstance(res.ValPk)
	require.NoError(t, err)
	require.True(t, valIns.IsRunning())
	require.NoError(t, err)

	// check validators on Babylon side
	require.Eventually(t, func() bool {
		vals, err := tm.BabylonClient.QueryValidators()
		if err != nil {
			t.Logf("failed to query validtors from Babylon %s", err.Error())
			return false
		}

		if len(vals) != 1 {
			return false
		}

		for _, v := range vals {
			if !strings.Contains(v.Description.Moniker, monikerPrefix) {
				return false
			}
			if !v.Commission.Equal(sdkmath.LegacyZeroDec()) {
				return false
			}
		}

		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	require.Equal(t, 1, len(app.ListValidatorInstances()))

	t.Logf("the test manager is running with a validator")

	return tm, valIns
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.Va.Stop()
	require.NoError(t, err)
	err = tm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.ValConfig.DatabaseConfig.Path)
	require.NoError(t, err)
	err = os.RemoveAll(tm.ValConfig.BabylonConfig.KeyDirectory)
	require.NoError(t, err)
	tm.EOTSServerHandler.Stop()
	err = os.RemoveAll(tm.EOTSServerHandler.baseDir)
	require.NoError(t, err)
	err = tm.CovenantEmulator.Stop()
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

	t.Logf("the validator is successfully registered")
}

func (tm *TestManager) WaitForValPubRandCommitted(t *testing.T, valIns *service.ValidatorInstance) {
	require.Eventually(t, func() bool {
		return valIns.GetLastCommittedHeight() > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("public randomness is successfully committed")
}

func (tm *TestManager) WaitForNPendingDels(t *testing.T, n int) []*types.Delegation {
	var (
		dels []*types.Delegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingDelegations(
			tm.CovenanConfig.DelegationLimit,
		)
		if err != nil {
			return false
		}
		return len(dels) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("delegations are pending")

	return dels
}

func (tm *TestManager) WaitForValNActiveDels(t *testing.T, btcPk *bbntypes.BIP340PubKey, n int) []*types.Delegation {
	var dels []*types.Delegation
	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.QueryStakingParams()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(btcPk, 1000)
		if err != nil {
			return false
		}
		return len(dels) == n && CheckDelsStatus(dels, currentBtcTip.Height, params.FinalizationTimeoutBlocks, bstypes.BTCDelegationStatus_ACTIVE)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the delegation is active, validators should start voting")

	return dels
}

func CheckDelsStatus(dels []*types.Delegation, btcHeight uint64, w uint64, status bstypes.BTCDelegationStatus) bool {
	allChecked := true
	for _, d := range dels {
		s := getDelStatus(d, btcHeight, w)
		if s != status {
			allChecked = false
		}
	}

	return allChecked
}

func getDelStatus(del *types.Delegation, btcHeight uint64, w uint64) bstypes.BTCDelegationStatus {
	if del.BtcUndelegation != nil {
		if del.BtcUndelegation.CovenantSlashingSigs != nil &&
			del.BtcUndelegation.CovenantUnbondingSigs != nil {
			return bstypes.BTCDelegationStatus_UNBONDED
		}
		// If we received an undelegation but is still does not have all required signature,
		// delegation receives UNBONING status.
		// Voting power from this delegation is removed from the total voting power and now we
		// are waiting for signatures from validator and covenant for delegation to become expired.
		// For now we do not have any unbonding time on the consumer chain, only time lock on BTC chain
		// we may consider adding unbonding time on the consumer chain later to avoid situation where
		// we can lose to much voting power in to short time.
		return bstypes.BTCDelegationStatus_UNBONDING
	}

	if del.StartHeight <= btcHeight && btcHeight+w <= del.EndHeight {
		if del.CovenantSigs != nil {
			return bstypes.BTCDelegationStatus_ACTIVE
		} else {
			return bstypes.BTCDelegationStatus_PENDING
		}
	}
	return bstypes.BTCDelegationStatus_UNBONDED
}

func (tm *TestManager) CheckBlockFinalization(t *testing.T, height uint64, num int) {
	// we need to ensure votes are collected at the given height
	require.Eventually(t, func() bool {
		votes, err := tm.BabylonClient.QueryVotesAtHeight(height)
		if err != nil {
			t.Logf("failed to get the votes at height %v: %s", height, err.Error())
			return false
		}
		return len(votes) == num
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// as the votes have been collected, the block should be finalized
	require.Eventually(t, func() bool {
		b, err := tm.BabylonClient.QueryBlock(height)
		if err != nil {
			t.Logf("failed to query block at height %v: %s", height, err.Error())
			return false
		}
		return b.Finalized
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) WaitForValVoteCast(t *testing.T, valIns *service.ValidatorInstance) uint64 {
	var lastVotedHeight uint64
	require.Eventually(t, func() bool {
		if valIns.GetLastVotedHeight() > 0 {
			lastVotedHeight = valIns.GetLastVotedHeight()
			return true
		} else {
			return false
		}
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return lastVotedHeight
}

func (tm *TestManager) WaitForNFinalizedBlocks(t *testing.T, n int) []*types.BlockInfo {
	var (
		blocks []*types.BlockInfo
		err    error
	)
	require.Eventually(t, func() bool {
		blocks, err = tm.BabylonClient.QueryLatestFinalizedBlocks(uint64(n))
		if err != nil {
			t.Logf("failed to get the latest finalized block: %s", err.Error())
			return false
		}
		return len(blocks) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the block is finalized at %v", blocks[0].Height)

	return blocks
}

func (tm *TestManager) StopAndRestartValidatorAfterNBlocks(t *testing.T, n int, valIns *service.ValidatorInstance) {
	blockBeforeStop, err := tm.BabylonClient.QueryBestBlock()
	require.NoError(t, err)
	err = valIns.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		headerAfterStop, err := tm.BabylonClient.QueryBestBlock()
		if err != nil {
			return false
		}

		return headerAfterStop.Height >= uint64(n)+blockBeforeStop.Height
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Log("restarting the validator instance")

	tm.ValConfig.ValidatorModeConfig.AutoChainScanningMode = true
	err = valIns.Start()
	require.NoError(t, err)
}

func (tm *TestManager) GetValPrivKey(t *testing.T, valPk []byte) *btcec.PrivateKey {
	record, err := tm.EOTSClient.KeyRecord(valPk, passphrase)
	require.NoError(t, err)
	return record.PrivKey
}

func (tm *TestManager) InsertBTCDelegation(t *testing.T, validatorPks []*btcec.PublicKey, stakingTime uint16, stakingAmount int64, params *types.StakingParams) *TestDelegationData {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	changeAddress, err := datagen.GenRandomBTCAddress(r, btcNetworkParams)
	require.NoError(t, err)

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		validatorPks,
		params.CovenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingAmount,
		params.SlashingAddress.String(), changeAddress.String(),
		params.SlashingRate,
	)

	// delegator Babylon key pairs
	delBabylonPrivKey, delBabylonPubKey, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)

	// proof-of-possession
	pop, err := bstypes.NewPoP(delBabylonPrivKey, delBtcPrivKey)
	require.NoError(t, err)

	// create and insert BTC headers which include the staking tx to get staking tx info
	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), testStakingInfo.StakingTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, *currentBtcTip.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: currentBtcTip.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]bbntypes.BTCHeaderBytes, 0)
	headers = append(headers, blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, *headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = tm.BabylonClient.InsertBtcBlockHeaders(headers)
	require.NoError(t, err)
	btcHeader := blockWithStakingTx.HeaderBytes
	serializedStakingTx, err := bbntypes.SerializeBTCTx(testStakingInfo.StakingTx)
	require.NoError(t, err)
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, serializedStakingTx, blockWithStakingTx.SpvProof.MerkleNodes)

	slashignSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		0,
		slashignSpendInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	// submit the BTC delegation to Babylon
	_, err = tm.BabylonClient.CreateBTCDelegation(
		delBabylonPubKey.(*secp256k1.PubKey),
		bbntypes.NewBIP340PubKeyFromBTCPK(delBtcPubKey),
		validatorPks,
		pop,
		uint32(stakingTime),
		stakingAmount,
		txInfo,
		testStakingInfo.SlashingTx,
		delegatorSig)
	require.NoError(t, err)

	t.Log("successfully submitted a BTC delegation")

	return &TestDelegationData{
		DelegatorPrivKey:        delBtcPrivKey,
		DelegatorKey:            delBtcPubKey,
		DelegatorBabylonPrivKey: delBabylonPrivKey.(*secp256k1.PrivKey),
		DelegatorBabylonKey:     delBabylonPubKey.(*secp256k1.PubKey),
		ValidatorPks:            validatorPks,
		StakingTx:               testStakingInfo.StakingTx,
		SlashingTx:              testStakingInfo.SlashingTx,
		StakingTxInfo:           txInfo,
		DelegatorSig:            delegatorSig,
		SlashingAddr:            params.SlashingAddress.String(),
		ChangeAddr:              changeAddress.String(),
		StakingTime:             stakingTime,
		StakingAmount:           stakingAmount,
	}
}

func (tm *TestManager) InsertBTCUnbonding(
	t *testing.T,
	actualDel *types.Delegation,
	delSK *btcec.PrivateKey,
	changeAddress string,
	params *types.StakingParams,
) {
	stakingMsgTx, _, err := bbntypes.NewBTCTxFromHex(actualDel.StakingTxHex)
	require.NoError(t, err)
	stakingTxHash := stakingMsgTx.TxHash()

	unbondingTime := uint16(params.FinalizationTimeoutBlocks) + 1
	unbondingValue := int64(actualDel.TotalSat - 1000)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delSK,
		actualDel.ValBtcPks,
		params.CovenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stakingTxHash, 0),
		unbondingTime,
		unbondingValue,
		params.SlashingAddress.String(), changeAddress,
		params.SlashingRate,
	)

	unbondingTxMsg := testUnbondingInfo.UnbondingTx

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	unbondingSig, err := testUnbondingInfo.SlashingTx.Sign(
		unbondingTxMsg,
		0,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	serializedUnbondingTx, err := bbntypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)

	_, err = tm.BabylonClient.CreateBTCUndelegation(
		serializedUnbondingTx, uint32(unbondingTime), unbondingValue, testUnbondingInfo.SlashingTx, unbondingSig,
	)
	require.NoError(t, err)

	t.Log("successfully submitted a BTC undelegation")
}

func (tm *TestManager) GetParams(t *testing.T) *types.StakingParams {
	p, err := tm.BabylonClient.QueryStakingParams()
	require.NoError(t, err)
	return p
}

func defaultValidatorConfig(keyringDir, testDir string) *valcfg.Config {
	cfg := valcfg.DefaultConfig()

	cfg.ValidatorModeConfig.AutoChainScanningMode = false
	// babylon configs for sending transactions
	cfg.BabylonConfig.KeyDirectory = keyringDir
	// need to use this one to send otherwise we will have account sequence mismatch
	// errors
	cfg.BabylonConfig.Key = "test-spending-key"
	// Big adjustment to make sure we have enough gas in our transactions
	cfg.BabylonConfig.GasAdjustment = 20
	cfg.DatabaseConfig.Path = filepath.Join(testDir, "db")
	cfg.UnbondingSigSubmissionInterval = 3 * time.Second

	return &cfg
}

func defaultCovenantConfig(testDir string) *covcfg.Config {
	cfg := covcfg.DefaultConfig()
	cfg.CovenantDir = testDir
	cfg.BabylonConfig.KeyDirectory = testDir

	return &cfg
}

func defaultEOTSConfig(t *testing.T) *eotsconfig.Config {
	cfg := eotsconfig.DefaultConfig()

	eotsDir, err := baseDir("zEOTSTest")
	require.NoError(t, err)

	configFile := filepath.Join(eotsDir, "covd-test.conf")
	dataDir := filepath.Join(eotsDir, "data")
	logDir := filepath.Join(eotsDir, "log")

	cfg.EOTSDir = eotsDir
	cfg.ConfigFile = configFile
	cfg.LogDir = logDir
	cfg.KeyDirectory = dataDir
	cfg.DatabaseConfig.Path = filepath.Join(eotsDir, "db")

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

func newDescription(moniker string) *stakingtypes.Description {
	dec := stakingtypes.NewDescription(moniker, "", "", "", "")
	return &dec
}
