//go:build e2e
// +build e2e

package e2etest

import (
	"os"
	"testing"
	"time"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
	cfg "github.com/babylonchain/btc-validator/valcfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

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
	defaultPollerConfig := cfg.DefaulPollerConfig()

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
