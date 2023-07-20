//go:build e2e
// +build e2e

package e2etest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
	defaultPollerConfig := valcfg.DefaulPollerConfig()

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

func TestCreateValidator(t *testing.T) {
	tDir, err := TempDirWithName("valtest")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tDir)
		require.NoError(t, err)
	}()

	handler, err := NewBabylonNodeHandler()
	require.NoError(t, err)

	err = handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	defaultConfig := valcfg.DefaultConfig()
	defaultConfig.BabylonConfig.KeyDirectory = handler.GetNodeDataDir()
	// need to use this one to send otherwise we will have account sequencemismatch
	// errors
	defaultConfig.BabylonConfig.Key = "test-spending-key"

	// Big adjustment to make sure we have enough gas in our transactions
	defaultConfig.BabylonConfig.GasAdjustment = 3.0

	defaultConfig.DatabaseConfig.Path = filepath.Join(tDir, "valtest.db")

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = os.Stdout

	bc, err := babylonclient.NewBabylonController(defaultConfig.BabylonConfig, logger)
	require.NoError(t, err)

	app, err := service.NewValidatorAppFromConfig(&defaultConfig, logger, bc)
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)
	defer app.Stop()

	newValName := "testingValidator"
	valResult, err := app.CreateValidator(newValName)
	require.NoError(t, err)

	validator, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)

	require.Equal(t, newValName, validator.KeyName)

	_, err = app.RegisterValidator(validator.BabylonPk)
	require.NoError(t, err)

	validatorAfterReg, err := app.GetValidator(valResult.BabylonValidatorPk.Key)
	require.NoError(t, err)
	require.Equal(t, validatorAfterReg.Status, proto.ValidatorStatus_VALIDATOR_STATUS_REGISTERED)

}
