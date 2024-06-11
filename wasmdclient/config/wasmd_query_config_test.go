package config_test

import (
	"testing"

	"github.com/babylonchain/finality-provider/wasmdclient/config"
	"github.com/stretchr/testify/require"
)

// TestWasmdQueryConfig ensures that the default Babylon query config is valid
func TestWasmdQueryConfig(t *testing.T) {
	defaultConfig := config.DefaultWasmdQueryConfig()
	err := defaultConfig.Validate()
	require.NoError(t, err)
}
