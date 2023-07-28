//go:build e2e
// +build e2e

package e2etest

import (
	"os"
	"testing"
	"time"

	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
)

// bitcoin params used for testing
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

	// Let the node start up
	// TODO Add polling with to check for startup
	// time.Sleep(5 * time.Second)
	defaultConfig := valcfg.DefaultBBNConfig()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout
	defaultPollerConfig := valcfg.DefaultChainPollerConfig()

	bc, err := babylonclient.NewBabylonController(&defaultConfig, logger)
	require.NoError(t, err)

	poller := service.NewChainPoller(logger, &defaultPollerConfig, bc)
	require.NoError(t, err)

	err = poller.Start()
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

func TestValidatorLifeCycle(t *testing.T) {
	tm := StartManager(t, false)
	defer tm.Stop(t)

	app := tm.Va
	newValName := "testingValidator"
	valResult, err := app.CreateValidator(newValName)
	require.NoError(t, err)

	validator, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)

	require.Equal(t, newValName, validator.KeyName)

	_, err = app.RegisterValidator(validator.KeyName)
	require.NoError(t, err)

	validatorAfterReg, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)
	require.Equal(t, validatorAfterReg.Status, proto.ValidatorStatus_REGISTERED)

	var queriedValidators []*btcstakingtypes.BTCValidator
	require.Eventually(t, func() bool {
		queriedValidators, err = tm.BabylonClient.QueryValidators()
		if err != nil {
			return false
		}
		return len(queriedValidators) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, queriedValidators[0].BabylonPk.Equals(validator.GetBabylonPK()))

	require.Eventually(t, func() bool {
		randPairs, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randPairs)
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func TestJurySigSubmission(t *testing.T) {
	tm := StartManager(t, true)
	defer tm.Stop(t)

	app := tm.Va
	newValName := "testingValidator"
	valResult, err := app.CreateValidator(newValName)
	require.NoError(t, err)

	validator, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)

	require.Equal(t, newValName, validator.KeyName)

	_, err = app.RegisterValidator(validator.KeyName)
	require.NoError(t, err)

	validatorAfterReg, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)
	require.Equal(t, validatorAfterReg.Status, proto.ValidatorStatus_REGISTERED)

	var queriedValidators []*btcstakingtypes.BTCValidator
	require.Eventually(t, func() bool {
		queriedValidators, err = tm.BabylonClient.QueryValidators()
		if err != nil {
			return false
		}
		return len(queriedValidators) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, queriedValidators[0].BabylonPk.Equals(validator.GetBabylonPK()))

	// send BTC delegation and make sure it's deep enough in btclightclient module
	delData := tm.InsertBTCDelegation(t, validator.MustGetBTCPK(), stakingTime, stakingAmount)

	var dels []*btcstakingtypes.BTCDelegation
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryPendingBTCDelegations()
		if err != nil {
			return false
		}
		return len(dels) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))

	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryActiveBTCValidatorDelegations(validator.MustGetBIP340BTCPK())
		if err != nil {
			return false
		}
		return len(dels) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	require.True(t, dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey))
}
