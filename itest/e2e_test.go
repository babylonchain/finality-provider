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
	"github.com/babylonchain/btc-validator/proto"
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
	tm := StartManager(t, false)
	defer tm.Stop(t)

	app := tm.Va
	newValName := "testingValidator"

	_, err := app.CreateValidator(newValName)
	require.NoError(t, err)
	_, bbnPk, err := app.RegisterValidator(newValName)
	require.NoError(t, err)
	err = app.StartHandlingValidator(bbnPk)
	require.NoError(t, err)
	valIns, err := app.GetValidatorInstance(bbnPk)
	require.NoError(t, err)
	require.Equal(t, valIns.GetStoreValidator().Status, proto.ValidatorStatus_REGISTERED)
	var queriedValidators []*btcstakingtypes.BTCValidator
	require.Eventually(t, func() bool {
		queriedValidators, err = tm.BabylonClient.QueryValidators()
		if err != nil {
			return false
		}
		return len(queriedValidators) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, queriedValidators[0].BabylonPk.Equals(valIns.GetBabylonPk()))

	// check the public randomness is committed
	require.Eventually(t, func() bool {
		randPairs, err := valIns.GetCommittedPubRandPairList()
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randPairs)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
	var dels []*btcstakingtypes.BTCDelegation
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
		if err != nil {
			return false
		}
		return len(dels) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	// check the BTC delegation is active
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}
		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	require.Eventually(t, func() bool {
		blocks, err := tm.BabylonClient.QueryLatestFinalizedBlocks(100)
		if err != nil {
			return false
		}
		if len(blocks) == 1 {
			return true
		}
		return false
	}, eventuallyWaitTimeOut, eventuallyPollTime)
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
		go func(v *service.ValidatorInstance) {
			// check the public randomness is committed
			require.Eventually(t, func() bool {
				randPairs, err := v.GetCommittedPubRandPairList()
				if err != nil {
					return false
				}
				return int(tm.Config.NumPubRand) == len(randPairs)
			}, eventuallyWaitTimeOut, eventuallyPollTime)

			// send a BTC delegation
			_ = tm.InsertBTCDelegation(t, v.MustGetBtcPk(), stakingTime, stakingAmount)
		}(valIns)
	}

	// check the BTC delegation is pending
	var (
		dels []*btcstakingtypes.BTCDelegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
		if err != nil {
			return false
		}
		return len(dels) == 3
	}, eventuallyWaitTimeOut*2, eventuallyPollTime)

	// submit Jury sigs for each delegation
	for _, del := range dels {
		go func(btcDel *btcstakingtypes.BTCDelegation) {
			_ = tm.AddJurySignature(t, btcDel)
		}(del)
	}

	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)

	for _, valIns := range valInstances {
		go func(v *service.ValidatorInstance) {
			// check the BTC delegation is active
			require.Eventually(t, func() bool {
				dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(v.GetBtcPkBIP340(), 1000)
				if err != nil {
					return false
				}
				status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
				return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
			}, eventuallyWaitTimeOut, eventuallyPollTime)
		}(valIns)
	}

	// check there's a block finalized
	require.Eventually(t, func() bool {
		blocks, err := tm.BabylonClient.QueryLatestFinalizedBlocks(100)
		if err != nil {
			return false
		}
		if len(blocks) == 1 {
			return true
		}
		return false
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func TestJurySigSubmission(t *testing.T) {
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
	require.Eventually(t, func() bool {
		randPairs, err := valIns.GetCommittedPubRandPairList()
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randPairs)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
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

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	// check the BTC delegation is active
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}
		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	var blocks []*types.BlockInfo
	require.Eventually(t, func() bool {
		blocks, err = tm.BabylonClient.QueryLatestFinalizedBlocks(100)
		if err != nil {
			return false
		}
		return len(blocks) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// attack: manually submit a finality vote over a conflicting block
	// to trigger the extraction of validator's private key
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := &types.BlockInfo{
		Height:         blocks[0].Height,
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

func TestValidatorUnbondingSigSubmission(t *testing.T) {
	tm := StartManagerWithValidator(t, 1, false)
	defer tm.Stop(t)

	app := tm.Va
	valIns := app.ListValidatorInstances()[0]

	// check the public randomness is committed
	require.Eventually(t, func() bool {
		randPairs, err := valIns.GetCommittedPubRandPairList()
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randPairs)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// send a BTC delegation
	delData := tm.InsertBTCDelegation(t, valIns.MustGetBtcPk(), stakingTime, stakingAmount)

	// check the BTC delegation is pending
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

	// submit Jury sig
	_ = tm.AddJurySignature(t, dels[0])

	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	params, err := tm.BabylonClient.GetStakingParams()
	require.NoError(t, err)
	// check the BTC delegation is active
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

	// wait for our validator to:
	// - detect new unbonding
	// - send signature
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPkBIP340(), 1000)
		if err != nil {
			return false
		}

		if len(dels) == 0 {
			return false
		}

		del := dels[0]

		if del.BtcUndelegation == nil {
			return false
		}

		if del.BtcUndelegation.ValidatorUnbondingSig == nil {
			return false
		}

		return true
	}, 1*time.Minute, eventuallyPollTime)
}
