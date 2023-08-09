package e2etest

import (
	"os"
	"testing"
	"time"

	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

var (
	stakingTime           = uint16(100)
	stakingAmount         = int64(20000)
	eventuallyWaitTimeOut = 10 * time.Second
	eventuallyPollTime    = 500 * time.Millisecond
)

func TempDirWithName(name string) (string, error) {
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

	bc, err := babylonclient.NewBabylonController(handler.GetNodeDataDir(), &defaultConfig, logger)
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
	tm := StartManagerWithoutValidator(t, false)
	defer tm.Stop(t)

	app := tm.Va
	newValName := "testingValidator"

	valResult, err := app.CreateValidator(newValName)
	require.NoError(t, err)
	_, err = app.RegisterValidator(newValName)
	require.NoError(t, err)
	valIns, err := app.GetValidatorInstance(&valResult.BabylonValidatorPk)
	require.NoError(t, err)
	require.Equal(t, valIns.GetValidatorStored().Status, proto.ValidatorStatus_REGISTERED)
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
	delData := tm.InsertBTCDelegation(t, valIns.GetBtcPk().MustToBTCPK(), stakingTime, stakingAmount)

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
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPk())
		if err != nil {
			return false
		}
		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	// check there's a block finalized
	require.Eventually(t, func() bool {
		blocks, err := tm.BabylonClient.QueryLatestFinalisedBlocks(100)
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
	delData := tm.InsertBTCDelegation(t, valIns.GetBtcPk().MustToBTCPK(), stakingTime, stakingAmount)

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
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valIns.GetBtcPk())
		if err != nil {
			return false
		}
		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))
}

//
//
// // TestDoubleSigning tests the attack scenario where the validator
// // sends a finality vote over a conflicting block
// // in this case, the BTC private key should be extracted by Babylon
// func TestDoubleSigning(t *testing.T) {
// 	tm := StartManagerWithoutValidator(t, false)
// 	defer tm.Stop(t)
//
// 	app := tm.Va
// 	newValName := "testingValidator"
//
// 	// create a validator object
// 	valResult, err := app.CreateValidator(newValName)
// 	require.NoError(t, err)
// 	validator, err := app.GetValidatorInstance(valResult.BabylonValidatorPk.Key)
// 	require.NoError(t, err)
// 	require.Equal(t, newValName, validator.KeyName)
//
// 	// register the validator to Babylon
// 	_, err = app.RegisterValidator(validator.KeyName)
// 	require.NoError(t, err)
// 	validatorAfterReg, err := app.GetValidatorInstance(valResult.BabylonValidatorPk.Key)
// 	require.NoError(t, err)
// 	require.Equal(t, validatorAfterReg.Status, proto.ValidatorStatus_REGISTERED)
// 	var queriedValidators []*btcstakingtypes.BTCValidator
// 	require.Eventually(t, func() bool {
// 		queriedValidators, err = tm.BabylonClient.QueryValidators()
// 		if err != nil {
// 			return false
// 		}
// 		return len(queriedValidators) == 1
// 	}, eventuallyWaitTimeOut, eventuallyPollTime)
// 	require.True(t, queriedValidators[0].BabylonPk.Equals(validator.GetBabylonPK()))
//
// 	// check the public randomness is committed
// 	require.Eventually(t, func() bool {
// 		randPairs, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
// 		if err != nil {
// 			return false
// 		}
// 		return int(tm.Config.NumPubRand) == len(randPairs)
// 	}, eventuallyWaitTimeOut, eventuallyPollTime)
//
// 	// send a BTC delegation
// 	delData := tm.InsertBTCDelegation(t, validator.MustGetBTCPK(), stakingTime, stakingAmount)
//
// 	// check the BTC delegation is pending
// 	var dels []*btcstakingtypes.BTCDelegation
// 	require.Eventually(t, func() bool {
// 		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
// 		if err != nil {
// 			return false
// 		}
// 		return len(dels) == 1
// 	}, eventuallyWaitTimeOut, eventuallyPollTime)
// 	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))
//
// 	// submit Jury sig
// 	_ = tm.AddJurySignature(t, dels[0])
//
// 	currentBtcTip, err := tm.BabylonClient.QueryBtcLightClientTip()
// 	require.NoError(t, err)
// 	params, err := tm.BabylonClient.GetStakingParams()
// 	require.NoError(t, err)
// 	// check the BTC delegation is active
// 	require.Eventually(t, func() bool {
// 		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(validator.MustGetBIP340BTCPK())
// 		if err != nil {
// 			return false
// 		}
// 		status := dels[0].GetStatus(currentBtcTip.Height, params.FinalizationTimeoutBlocks)
// 		return len(dels) == 1 && status == btcstakingtypes.BTCDelegationStatus_ACTIVE
// 	}, eventuallyWaitTimeOut, eventuallyPollTime)
// 	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))
//
// 	// check there's a block finalized
// 	var blocks []*types.IndexedBlock
// 	require.Eventually(t, func() bool {
// 		blocks, err = tm.BabylonClient.QueryLatestFinalisedBlocks(100)
// 		if err != nil {
// 			return false
// 		}
// 		if len(blocks) == 1 {
// 			return true
// 		}
// 		return false
// 	}, eventuallyWaitTimeOut, eventuallyPollTime)
//
// 	// attack: manually submit a finality vote over a conflicting block
// 	// to trigger the extraction of validator's private key
// 	r := rand.New(rand.NewSource(time.Now().UnixNano()))
// 	b := &service.BlockInfo{
// 		Height:         blocks[0].Height,
// 		LastCommitHash: datagen.GenRandomLastCommitHash(r),
// 	}
// 	_, extractedKey, err := app.SubmitFinalitySignatureForValidator(b, validator)
// 	require.NoError(t, err)
// 	require.NotNil(t, extractedKey)
// 	localKey, err := getBtcPrivKey(app.GetKeyring(), val.KeyName(validator.KeyName))
// 	require.NoError(t, err)
// 	require.True(t, localKey.Key.Equals(&extractedKey.Key) || localKey.Key.Negate().Equals(&extractedKey.Key))
// }

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
