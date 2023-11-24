package e2etest

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
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
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/eotsmanager/client"
	eotsconfig "github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
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
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *bbntypes.BIP340Signature

	SlashingAddr  string
	StakingTime   uint16
	StakingAmount int64
}

func StartManager(t *testing.T) *TestManager {
	testDir, err := tempDirWithName("vale2etest")
	require.NoError(t, err)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout

	// 1. prepare covenant key, which will be used as input of Babylon node
	covenantConfig := defaultCovenantConfig(testDir)
	err = covenantConfig.Validate()
	require.NoError(t, err)
	covenantPk, err := covenant.CreateCovenantKey(testDir, chainID, covenantKeyName, keyring.BackendTest, passphrase, hdPath)
	require.NoError(t, err)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, bbntypes.NewBIP340PubKeyFromBTCPK(covenantPk))
	err = bh.Start()
	require.NoError(t, err)
	cfg := defaultValidatorConfig(bh.GetNodeDataDir(), testDir)
	bc, err := clientcontroller.NewBabylonController(bh.GetNodeDataDir(), cfg.BabylonConfig, logger)
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

func StartManagerWithValidator(t *testing.T, n int) *TestManager {
	tm := StartManager(t)
	app := tm.Va

	for i := 0; i < n; i++ {
		valName := valNamePrefix + strconv.Itoa(i)
		moniker := monikerPrefix + strconv.Itoa(i)
		commission := sdktypes.DecCoin{Denom: "bbn", Amount: sdkmath.LegacyZeroDec()}
		res, err := app.CreateValidator(valName, chainID, passphrase, hdPath, newDescription(moniker), &commission)
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

			if len(vals) != i+1 {
				return false
			}

			for _, v := range vals {
				if !strings.Contains(v.Description.Moniker, monikerPrefix) {
					return false
				}
				if !v.Commission.Equal(commission.Amount) {
					return false
				}
			}

			return true
		}, eventuallyWaitTimeOut, eventuallyPollTime)
	}

	require.Equal(t, n, len(app.ListValidatorInstances()))

	t.Logf("the test manager is running with %v validators", n)

	return tm
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

func (tm *TestManager) WaitForValNUnbondingDels(t *testing.T, btcPk *bbntypes.BIP340PubKey, n int) []*types.Delegation {
	var (
		dels []*types.Delegation
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

		return len(dels) == 1 && dels[0].BtcUndelegation != nil

	}, 1*time.Minute, eventuallyPollTime)

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
		if del.BtcUndelegation.CovenantSlashingSig != nil &&
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
		if del.CovenantSig != nil {
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

func (tm *TestManager) WaitForValStopped(t *testing.T, valPk *bbntypes.BIP340PubKey) {
	require.Eventually(t, func() bool {
		_, err := tm.Va.GetValidatorInstance(valPk)
		return err != nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
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

func (tm *TestManager) InsertBTCDelegation(t *testing.T, valBtcPk *btcec.PublicKey, stakingTime uint16, stakingAmount int64) *TestDelegationData {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// examine staking params
	params, err := tm.BabylonClient.QueryStakingParams()
	slashingAddr := params.SlashingAddress
	require.NoError(t, err)
	require.Equal(t, tm.BabylonHandler.GetSlashingAddress(), slashingAddr)
	require.Greater(t, stakingTime, uint16(params.ComfirmationTimeBlocks))
	covenantPk := tm.BabylonHandler.GetCovenantPk().MustToBTCPK()
	require.NoError(t, err)
	require.Equal(t, params.CovenantPk.SerializeCompressed()[1:], covenantPk.SerializeCompressed()[1:])

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// validator BTC key pairs, staking tx and slashing tx
	valBtcPrivKey, valBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// generate staking tx and slashing tx
	stakingTimeBlocks := uint16(math.MaxUint16)
	testStakingInfo := datagen.GenBTCStakingSlashingTx(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		[]*btcec.PublicKey{valBtcPubKey},
		covenantBTCPKs,
		1,
		stakingTimeBlocks,
		stakingValue,
		params.SlashingAddress, changeAddress.EncodeAddress(),
		params.SlashingRate,
	)
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
	params, err := tm.BabylonClient.QueryStakingParams()
	require.NoError(t, err)
	slashingAddr := params.SlashingAddress
	stakingMsgTx, err := stakingTx.ToMsgTx()
	require.NoError(t, err)
	stakingTxChainHash := stakingTx.MustGetTxHash()
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
		params.CovenantPk,
		wire.NewOutPoint(&stakingTxChainHash, uint32(stakingOutputIdx)),
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
