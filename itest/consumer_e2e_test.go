package e2etest

import (
	"fmt"
	"testing"
)

// TestConsumerFinalityProviderRegistration tests finality-provider registration for a consumer chain
func TestConsumer(t *testing.T) {
	_ = StartManager(t)
	fmt.Println("TestConsumer")

	//consumerChainID := "consumer-chain-test-1"
	//
	//_, err := tm.BBNClient.RegisterConsumerChain(consumerChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	//require.NoError(t, err)
	//
	//tm.CreateFinalityProvidersForChain(t, consumerChainID, 1)
}
