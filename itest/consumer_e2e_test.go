//go:build e2e
// +build e2e

package e2etest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConsumerStoreContract stores a contract in the consumer chain
func TestConsumerStoreContract(t *testing.T) {
	tm, _, _ := StartManagerWithFinalityProvider(t, 1)
	defer tm.Stop(t)
	// Store the Babylon contract in the consumer chain
	babylonContractPath := "wasmd_contracts/babylon_contract.wasm"
	storedCodeID, _, err := tm.WasmdHandler.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
	// Query the latest code ID from "wasmd q wasm list-code"
	latestCodeId, err := tm.WasmdHandler.GetLatestCodeID()
	require.NoError(t, err)
	// Assert that the code id returned from store-code and list-code is the same
	require.Equal(t, storedCodeID, latestCodeId)
}
