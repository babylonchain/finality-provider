//go:build e2e
// +build e2e

package e2etest

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
)

// bitcoin params used for testing
var (
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
	handler, err := NewBabylonNodeHandler()
	require.NoError(t, err)

	err = handler.Start()
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

	require.Eventually(t, func() bool {
		randParis, err := app.GetCommittedPubRandPairList(validator.BabylonPk)
		if err != nil {
			return false
		}
		return int(tm.Config.NumPubRand) == len(randParis)
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

	// send BTC delegation
	delData := tm.InsertBTCDelegation(t, validator.MustGetBTCPK())

	require.Eventually(t, func() bool {
		dels, err := app.GetPendingDelegationsForAll()
		if err != nil {
			return false
		}
		return len(dels) == 1 && dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	require.Eventually(t, func() bool {
		dels, err := tm.BabylonClient.QueryActiveBTCValidatorDelegations(validator.MustGetBIP340BTCPK())
		if err != nil {
			return false
		}
		return len(dels) == 1 && dels[0].BabylonPk.Equals(delData.DelegatorBabylonKey)
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}
