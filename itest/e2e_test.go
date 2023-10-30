//go:build e2e
// +build e2e

package e2etest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
)

var (
	stakingTime   = uint16(100)
	stakingAmount = int64(20000)
)

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
	_ = tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	// check the BTC delegation is active
	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)
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
		go func(btcDel *types.Delegation) {
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
	_ = tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	// check the BTC delegation is active
	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)

	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)

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
	localKey := tm.GetValPrivKey(t, valIns.GetBtcPkBIP340().MustMarshal())
	require.True(t, localKey.Key.Equals(&extractedKey.Key) || localKey.Key.Negate().Equals(&extractedKey.Key))

	t.Logf("the equivocation attack is successful")
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
	_ = tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	dels := tm.WaitForNPendingDels(t, 1)

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	dels = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)

	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)

	n := 3
	// stop the validator for a few blocks then restart to trigger the fast sync
	tm.ValConfig.FastSyncGap = uint64(n)
	tm.StopAndRestartValidatorAfterNBlocks(t, n, valIns)

	// check there are n+1 blocks finalized
	finalizedBlocks = tm.WaitForNFinalizedBlocks(t, n+1)
	finalizedHeight := finalizedBlocks[0].Height
	t.Logf("the latest finalized block is at %v", finalizedHeight)

	// check if the fast sync works by checking if the gap is not more than 1
	currentHeaderRes, err := tm.BabylonClient.QueryBestBlock()
	currentHeight := currentHeaderRes.Height
	t.Logf("the current block is at %v", currentHeight)
	require.NoError(t, err)
	require.True(t, currentHeight < finalizedHeight+uint64(n))
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

	tm.InsertBTCUnbonding(t, delData.StakingTx, delData.DelegatorPrivKey, valIns.MustGetBtcPk())

	_ = tm.WaitForValNUnbondingDels(t, valIns.GetBtcPkBIP340(), 1)
}

func TestJuryLifeCycle(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, true)
	defer tm.Stop(t)
	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// send BTC delegation and make sure it's deep enough in btclightclient module
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	dels := tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	err := valIns.Stop()
	require.NoError(t, err)

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

		return del.BtcUndelegation.JurySlashingSig != nil && del.BtcUndelegation.JuryUnbondingSig != nil
	}, 1*time.Minute, eventuallyPollTime)
}
