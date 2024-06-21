//go:build e2e_op
// +build e2e_op

package e2etest

import (
	"testing"
)

// TestSubmitFinalitySignature tests the finality signature submission to the btc staking contract using admin
func TestSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// // store btc staking contract
	// btcStakingContractPath := "bytecode/btc_staking.wasm"
	// err = ctm.WasmdConsumerClient.StoreWasmCode(btcStakingContractPath)
	// require.NoError(t, err)
	// btcStakingContractWasmId, err := ctm.WasmdConsumerClient.GetLatestCodeId()
	// require.NoError(t, err)
	// require.Equal(t, uint64(2), btcStakingContractWasmId)

	// // instantiate babylon contract with admin
	// btcStakingInitMsg := map[string]interface{}{
	// 	"admin": ctm.WasmdConsumerClient.MustGetValidatorAddress(),
	// }
	// btcStakingInitMsgBytes, err := json.Marshal(btcStakingInitMsg)
	// require.NoError(t, err)
	// initMsg := map[string]interface{}{
	// 	"network":                         "regtest",
	// 	"babylon_tag":                     "01020304",
	// 	"btc_confirmation_depth":          1,
	// 	"checkpoint_finalization_timeout": 2,
	// 	"notify_cosmos_zone":              false,
	// 	"btc_staking_code_id":             btcStakingContractWasmId,
	// 	"btc_staking_msg":                 btcStakingInitMsgBytes,
	// 	"admin":                           ctm.WasmdConsumerClient.MustGetValidatorAddress(),
	// }
	// initMsgBytes, err := json.Marshal(initMsg)
	// require.NoError(t, err)
	// err = ctm.WasmdConsumerClient.InstantiateContract(babylonContractWasmId, initMsgBytes)
	// require.NoError(t, err)

}
