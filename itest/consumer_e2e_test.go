package e2etest

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	e2etypes "github.com/babylonchain/finality-provider/itest/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

// TODO: currently fp app is not started in consumer manager, so the following tests are commented out
//
//	uncomment after contract is ready and fp app is working
func TestConsumerFinalityProviderRegistration(t *testing.T) {
	ctm, _ := StartConsumerManagerWithFps(t, 1)
	defer ctm.Stop(t)

	consumerChainID := "consumer-chain-test-1"
	_, err := ctm.BBNClient.RegisterConsumerChain(consumerChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	require.NoError(t, err)

	ctm.CreateFinalityProvidersForChain(t, consumerChainID, 1)
}

// TODO: make a test suite for the wasmd <-> babylon e2e tests
// TestSubmitFinalitySignature tests the finality signature submission to the btc staking contract using admin
func TestSubmitFinalitySignature(t *testing.T) {
	ctm := StartConsumerManager(t)
	defer ctm.Stop(t)

	// store babylon contract
	babylonContractPath := "bytecode/babylon_contract.wasm"
	err := ctm.WasmdConsumerClient.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
	babylonContractWasmId, err := ctm.WasmdConsumerClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(1), babylonContractWasmId)

	// store btc staking contract
	btcStakingContractPath := "bytecode/btc_staking.wasm"
	err = ctm.WasmdConsumerClient.StoreWasmCode(btcStakingContractPath)
	require.NoError(t, err)
	btcStakingContractWasmId, err := ctm.WasmdConsumerClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(2), btcStakingContractWasmId)

	// instantiate babylon contract with admin
	btcStakingInitMsg := map[string]interface{}{
		"admin": ctm.WasmdConsumerClient.CosmwasmClient.MustGetAddr(),
	}
	btcStakingInitMsgBytes, err := json.Marshal(btcStakingInitMsg)
	require.NoError(t, err)
	initMsg := map[string]interface{}{
		"network":                         "regtest",
		"babylon_tag":                     "01020304",
		"btc_confirmation_depth":          1,
		"checkpoint_finalization_timeout": 2,
		"notify_cosmos_zone":              false,
		"btc_staking_code_id":             btcStakingContractWasmId,
		"btc_staking_msg":                 btcStakingInitMsgBytes,
		"admin":                           ctm.WasmdConsumerClient.CosmwasmClient.MustGetAddr(),
	}
	initMsgBytes, err := json.Marshal(initMsg)
	require.NoError(t, err)
	err = ctm.WasmdConsumerClient.InstantiateContract(babylonContractWasmId, initMsgBytes)
	require.NoError(t, err)

	// get btc staking contract address
	resp, err := ctm.WasmdConsumerClient.ListContractsByCode(btcStakingContractWasmId, &sdkquerytypes.PageRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Contracts, 1)
	btcStakingContractAddr := sdk.MustAccAddressFromBech32(resp.Contracts[0])

	// generate ibc packet and send to btc staking contract using admin
	//r := rand.New(rand.NewSource(time.Now().UnixNano()))
	//msg := GenIBCPacket(t, r)
	//msgBytes, err := zctypes.ModuleCdc.MarshalJSON(msg)
	//require.NoError(t, err)
	msg := e2etypes.GenExecMessage()
	msgBytes, err := json.Marshal(msg)
	require.NoError(t, err)
	err = ctm.WasmdConsumerClient.Exec(btcStakingContractAddr, msgBytes)
	require.NoError(t, err)

	// query finality providers in smart contract
	dataFromContract, err := ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"finality_providers": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var consumerFps e2etypes.ConsumerFpsResponse
	err = json.Unmarshal(dataFromContract.Data, &consumerFps)
	require.NoError(t, err)
	require.Len(t, consumerFps.ConsumerFps, 1)
	require.Equal(t, msg.BtcStaking.NewFP[0].ConsumerID, consumerFps.ConsumerFps[0].ConsumerId)
	require.Equal(t, msg.BtcStaking.NewFP[0].BTCPKHex, consumerFps.ConsumerFps[0].BtcPkHex)

	// query delegations in smart contract
	dataFromContract, err = ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"delegations": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var consumerDels e2etypes.ConsumerDelegationsResponse
	err = json.Unmarshal(dataFromContract.Data, &consumerDels)
	require.NoError(t, err)
	require.Len(t, consumerDels.ConsumerDelegations, 1)
	require.Empty(t, consumerDels.ConsumerDelegations[0].UndelegationInfo.DelegatorUnbondingSig) // assert there is no delegator unbonding sig
	require.Equal(t, msg.BtcStaking.ActiveDel[0].BTCPkHex, consumerDels.ConsumerDelegations[0].BtcPkHex)
	require.Equal(t, msg.BtcStaking.ActiveDel[0].StartHeight, consumerDels.ConsumerDelegations[0].StartHeight)
	require.Equal(t, msg.BtcStaking.ActiveDel[0].EndHeight, consumerDels.ConsumerDelegations[0].EndHeight)
	require.Equal(t, msg.BtcStaking.ActiveDel[0].TotalSat, consumerDels.ConsumerDelegations[0].TotalSat)
	require.Equal(t, msg.BtcStaking.ActiveDel[0].StakingTx, base64.StdEncoding.EncodeToString(consumerDels.ConsumerDelegations[0].StakingTx))   // make sure to compare b64 encoded strings
	require.Equal(t, msg.BtcStaking.ActiveDel[0].SlashingTx, base64.StdEncoding.EncodeToString(consumerDels.ConsumerDelegations[0].SlashingTx)) // make sure to compare b64 encoded strings

	// submit finality signature to the btc staking contract using admin
	//finalitySigMsg := GenFinalitySignatureMessage(msg.Packet.(*zctypes.ZoneconciergePacketData_BtcStaking).BtcStaking.NewFp[0].BtcPkHex)
	//finalitySigMsgBytes, err := json.Marshal(finalitySigMsg)
	//require.NoError(t, err)
	//err = ctm.WasmdConsumerClient.Exec(btcStakingContractAddr, finalitySigMsgBytes)
	//// TODO: insert delegation and pub randomness to fix the error
	//require.Error(t, err)
}
