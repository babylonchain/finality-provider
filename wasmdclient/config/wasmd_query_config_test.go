package config_test

import (
	"testing"

	"github.com/babylonchain/babylon/client/config"
	"github.com/stretchr/testify/require"
)

// TestCosmosChainQueryConfig ensures that the default Babylon query config is valid
func TestCosmosChainQueryConfig(t *testing.T) {
	defaultConfig := config.DefaultBabylonQueryConfig()
	err := defaultConfig.Validate()
	require.NoError(t, err)
}
