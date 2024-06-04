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

	babylonContractPath := "wasmd_contracts/babylon_contract.wasm"
	_, _, err := tm.WasmdHandler.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
}
