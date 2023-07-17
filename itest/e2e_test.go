//go:build e2e
// +build e2e

package e2etest

import (
	"testing"
	"time"

	"github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/rpc-client/config"
	"github.com/stretchr/testify/require"
)

// TODO For now tests checks only if test works. Add something more sensible
func TestBabylonNodeProgressing(t *testing.T) {
	handler, err := NewBabylonNodeHandler()
	require.NoError(t, err)

	err = handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	// Let the node start up
	// TODO Add polling with to check for startup
	// time.Sleep(5 * time.Second)
	defaultConfig := config.DefaultBabylonConfig()
	client, err := client.New(&defaultConfig)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		resp, err := client.QueryClient.GetStatus()
		if err != nil {
			return false
		}

		// Wait for 3 blocks to check node is progressing as expected
		if resp.SyncInfo.LatestBlockHeight < 3 {
			return false
		}

		return true
	}, 30*time.Second, 1*time.Second)
}
