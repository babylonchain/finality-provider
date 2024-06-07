//go:build e2e
// +build e2e

package e2etest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConsumerFinalityProviderRegistration tests finality-provider registration for a consumer chain
func TestConsumerFinalityProviderRegistration(t *testing.T) {
	ctm, _ := StartConsumerManagerWithFps(t, 1)
	defer ctm.Stop(t)

	consumerChainID := "consumer-chain-test-1"
	_, err := ctm.BBNClient.RegisterConsumerChain(consumerChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	require.NoError(t, err)

	ctm.CreateFinalityProvidersForChain(t, consumerChainID, 1)
}

// TestConsumerStoreContract stores a contract in the consumer chain
func TestConsumerStoreContract(t *testing.T) {
	ctm, _ := StartConsumerManagerWithFps(t, 1)
	defer ctm.Stop(t)

	// Store the Babylon contract in the consumer chain
	babylonContractPath := "bytecode/babylon_contract.wasm"
	storedCodeID, _, err := ctm.WasmdHandler.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
	// Query the latest code ID from "wasmd q wasm list-code"
	latestCodeID, err := ctm.WasmdHandler.GetLatestCodeID()
	require.NoError(t, err)
	// Assert that the code id returned from store-code and list-code is the same
	require.Equal(t, storedCodeID, latestCodeID)

	bb, _ := ctm.WasmdConsumerClient.QueryCometBestBlock()
	fmt.Print(bb.Height)
}
