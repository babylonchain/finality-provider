//go:build e2e
// +build e2e

package e2etest

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	stakingTime   = uint16(100)
	stakingAmount = int64(20000)
)

func TestPoller(t *testing.T) {
	handler := NewBabylonNodeHandler(t)

	err := handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	defaultConfig := valcfg.DefaultBBNConfig()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout
	defaultPollerConfig := valcfg.DefaultChainPollerConfig()

	bc, err := clientcontroller.NewBabylonController(handler.GetNodeDataDir(), &defaultConfig, logger)
	require.NoError(t, err)

	poller := service.NewChainPoller(logger, &defaultPollerConfig, bc)
	require.NoError(t, err)

	// Set auto calculated start height to 1, as we have disabled automatic start height calculation
	err = poller.Start(1)
	require.NoError(t, err)
	defer poller.Stop()

	// Get 3 blocks which should be received in order
	select {
	case info := <-poller.GetBlockInfoChan():
		require.Equal(t, uint64(1), info.Height)

	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to get block info")
	}

	select {
	case info := <-poller.GetBlockInfoChan():
		require.Equal(t, uint64(2), info.Height)

	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to get block info")
	}

	select {
	case info := <-poller.GetBlockInfoChan():
		require.Equal(t, uint64(3), info.Height)

	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to get block info")
	}
}

// TestValidatorLifeCycle tests the whole life cycle of a validator
// creation -> registration -> randomness commitment ->
// activation with BTC delegation and Jury sig ->
// vote submission -> block finalization
func TestValidatorLifeCycle(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, false)
	defer tm.Stop(t)

	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	// check the BTC delegation is active
	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)
	t.Logf("the latest finalized block is at %v", finalizedBlocks[0].Height)
}

// TestMultipleValidators tests starting with multiple validators
func TestMultipleValidators(t *testing.T) {
	n := 3
	tm := StartManagerWithValidator(t, n, false)
	defer tm.Stop(t)

	app := tm.Va
	valInstances := app.ListValidatorInstances()

	// submit BTC delegations for each validator
	for _, valIns := range valInstances {
		tm.Wg.Add(1)
		go func(v *service.ValidatorInstance) {
			defer tm.Wg.Done()
			// check the public randomness is committed
			tm.WaitForValPubRandCommitted(t, v)

			// send a BTC delegation
			_ = tm.InsertBTCDelegation(t, v.MustGetBtcPk(), stakingTime, stakingAmount)
		}(valIns)
	}
	tm.Wg.Wait()

	// check the 3 BTC delegations are pending
	dels := tm.WaitForNPendingDels(t, 3)

	// submit Jury sigs for each delegation
	for _, del := range dels {
		tm.Wg.Add(1)
		go func(btcDel *btcstakingtypes.BTCDelegation) {
			defer tm.Wg.Done()
			_ = tm.AddJurySignature(t, btcDel)
		}(del)
	}
	tm.Wg.Wait()

	for _, valIns := range valInstances {
		tm.Wg.Add(1)
		go func(v *service.ValidatorInstance) {
			defer tm.Wg.Done()
			_ = tm.WaitForValNActiveDels(t, v.GetBtcPkBIP340(), 1)
		}(valIns)
	}
	tm.Wg.Wait()

	// check there's a block finalized
	_ = tm.WaitForNFinalizedBlocks(t, 1)
}

func TestJurySigSubmission(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, true)
	defer tm.Stop(t)
	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// send BTC delegation and make sure it's deep enough in btclightclient module
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	dels := tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))
}

// TestDoubleSigning tests the attack scenario where the validator
// sends a finality vote over a conflicting block
// in this case, the BTC private key should be extracted by Babylon
func TestDoubleSigning(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, false)
	defer tm.Stop(t)

	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	// check the BTC delegation is active
	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)
	t.Logf("the latest finalized block is at %v", finalizedBlocks[0].Height)

	// attack: manually submit a finality vote over a conflicting block
	// to trigger the extraction of validator's private key
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := &types.BlockInfo{
		Height:         finalizedBlocks[0].Height,
		LastCommitHash: datagen.GenRandomLastCommitHash(r),
	}
	_, extractedKey, err := valIns.TestSubmitFinalitySignatureAndExtractPrivKey(b)
	require.NoError(t, err)
	require.NotNil(t, extractedKey)
	localKey, err := getBtcPrivKey(app.GetKeyring(), val.KeyName(valIns.GetStoreValidator().KeyName))
	require.NoError(t, err)
	require.True(t, localKey.Key.Equals(&extractedKey.Key) || localKey.Key.Negate().Equals(&extractedKey.Key))
}

func getBtcPrivKey(kr keyring.Keyring, keyName val.KeyName) (*btcec.PrivateKey, error) {
	k, err := kr.Key(keyName.GetBtcKeyName())
	if err != nil {
		return nil, err
	}
	localKey := k.GetLocal().PrivKey.GetCachedValue()
	switch v := localKey.(type) {
	case *secp256k1.PrivKey:
		privKey, _ := btcec.PrivKeyFromBytes(v.Key)
		return privKey, nil
	default:
		return nil, err
	}
}

// TestFastSync tests the fast sync process where the validator is terminated and restarted with fast sync
func TestFastSync(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, false)
	defer tm.Stop(t)

	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)
	t.Logf("the latest finalized block is at %v", finalizedBlocks[0].Height)

	n := 3
	// stop the validator for a few blocks then restart to trigger the fast sync
	tm.Config.FastSyncGap = uint64(n)
	tm.StopAndRestartValidatorAfterNBlocks(t, n, valIns)

	// check there are n+1 blocks finalized
	finalizedBlocks = tm.WaitForNFinalizedBlocks(t, n+1)
	finalizedHeight := finalizedBlocks[0].Height
	t.Logf("the latest finalized block is at %v", finalizedHeight)

	// check if the fast sync works by checking if the gap is not more than 1
	currentHeaderRes, err := tm.BabylonClient.QueryBestHeader()
	currentHeight := currentHeaderRes.Header.Height
	t.Logf("the current block is at %v", currentHeight)
	require.NoError(t, err)
	require.True(t, currentHeight < int64(finalizedHeight)+int64(n))
}

func TestValidatorUnbondingSigSubmission(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, false)
	defer tm.Stop(t)

	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	tm.InsertBTCUnbonding(t, delData.StakingTx, delData.DelegatorPrivKey, valIns.MustGetBtcPk())

	_ = tm.WaitForValNUnbondingDels(t, valIns.GetBtcPkBIP340(), 1)
}

func TestJuryUnbondingSigSubmission(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, true)
	defer tm.Stop(t)
	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// send BTC delegation and make sure it's deep enough in btclightclient module
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	var (
		dels []*btcstakingtypes.BTCDelegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
		if err != nil {
			return false
		}
		return len(dels) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}
		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	tm.InsertBTCUnbonding(t, delData.StakingTx, delData.DelegatorPrivKey, valIns.MustGetBtcPk())

	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}
		return len(dels) == 1 && dels[0].BtcUndelegation != nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
	require.NoError(t, err)
	delegationWithUndelegation := dels[0]

	validatorPrivKey, err := valIns.BtcPrivKey()
	require.NoError(t, err)

	tm.AddValidatorUnbondingSignature(
		t,
		delegationWithUndelegation,
		validatorPrivKey,
	)

	// after providing validator unbodning signature, we should wait for jury to provide both valid signatures
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}

		if len(dels) != 1 {
			return false
		}

		del := dels[0]

		if del.BtcUndelegation == nil {
			return false
		}

		return del.BtcUndelegation.HasJurySigs()
	}, 1*time.Minute, eventuallyPollTime)
}
