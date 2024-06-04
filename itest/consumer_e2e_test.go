package e2etest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConsumerFinalityProviderRegistration tests finality-provider registration for a consumer chain
func TestConsumer(t *testing.T) {
	tm := StartManager(t)
	defer tm.Stop(t)
	codeID, _, err := tm.WasmdHandler.StoreWasmCode(t, "/Users/gusin/Github/finality-provider/itest/wasmd_contracts/babylon_contract.wasm")
	require.NoError(t, err)
	fmt.Println(codeID)

	//consumerChainID := "consumer-chain-test-1"
	//
	//_, err := tm.BBNClient.RegisterConsumerChain(consumerChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	//require.NoError(t, err)
	//
	//tm.CreateFinalityProvidersForChain(t, consumerChainID, 1)
}
