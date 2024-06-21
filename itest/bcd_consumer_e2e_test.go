package e2etest

import (
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

func TestConsumerFinalityProviderRegistration(t *testing.T) {
	ctm := StartBcdTestManager(t)
	defer ctm.Stop(t)

	_, err := ctm.BBNClient.RegisterConsumerChain(bcdChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	require.NoError(t, err)

	ctm.CreateConsumerFinalityProviders(t, bcdChainID, 1)
}

// TestSubmitFinalitySignature tests the finality signature submission to the btc staking contract using admin
func TestSubmitFinalitySignature2(t *testing.T) {
	ctm := StartBcdTestManager(t)
	defer ctm.Stop(t)

	// store babylon contract
	babylonContractPath := "bytecode/babylon_contract.wasm"
	err := ctm.BcdConsumerClient.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
	babylonContractWasmId, err := ctm.BcdConsumerClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(1), babylonContractWasmId)

	// store btc staking contract
	btcStakingContractPath := "bytecode/btc_staking.wasm"
	err = ctm.BcdConsumerClient.StoreWasmCode(btcStakingContractPath)
	require.NoError(t, err)
	btcStakingContractWasmId, err := ctm.BcdConsumerClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(2), btcStakingContractWasmId)

	// instantiate babylon contract with admin
	btcStakingInitMsg := map[string]interface{}{
		"admin": ctm.BcdConsumerClient.MustGetValidatorAddress(),
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
		"admin":                           ctm.BcdConsumerClient.MustGetValidatorAddress(),
	}
	initMsgBytes, err := json.Marshal(initMsg)
	require.NoError(t, err)
	err = ctm.BcdConsumerClient.InstantiateContract(babylonContractWasmId, initMsgBytes)
	require.NoError(t, err)

	// get btc staking contract address
	resp, err := ctm.BcdConsumerClient.ListContractsByCode(btcStakingContractWasmId, &sdkquerytypes.PageRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Contracts, 1)
	btcStakingContractAddr := sdk.MustAccAddressFromBech32(resp.Contracts[0])
	// update the contract address in config because during setup we had used a random address which is not valid
	ctm.BcdConsumerClient.SetBtcStakingContractAddress(btcStakingContractAddr.String())

	_, err = ctm.BBNClient.RegisterConsumerChain(bcdChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	require.NoError(t, err)

	ctm.CreateConsumerFinalityProviders(t, bcdChainID, 1)
	/*
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		fpSk, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		randList, msgPub, err := GenCommitPubRandListMsg(r, fpSk, 1, 1000)
		require.NoError(t, err)

		// inject fp and delegation in smart contract using admin
		msg := GenBtcStakingExecMsg(msgPub.FpBtcPk.MarshalHex())
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)
		_, err = ctm.BcdConsumerClient.ExecuteContract(msgBytes)
		require.NoError(t, err)

		// query finality providers in smart contract
		consumerFpsResp, err := ctm.BcdConsumerClient.QueryFinalityProviders()
		require.NoError(t, err)
		require.NotNil(t, consumerFpsResp)
		require.Len(t, consumerFpsResp.Fps, 1)
		require.Equal(t, msg.BtcStaking.NewFP[0].ConsumerID, consumerFpsResp.Fps[0].ConsumerId)
		require.Equal(t, msg.BtcStaking.NewFP[0].BTCPKHex, consumerFpsResp.Fps[0].BtcPkHex)

		// query delegations in smart contract
		consumerDelsResp, err := ctm.BcdConsumerClient.QueryDelegations()
		require.NoError(t, err)
		require.NotNil(t, consumerDelsResp)
		require.Len(t, consumerDelsResp.Delegations, 1)
		require.Empty(t, consumerDelsResp.Delegations[0].UndelegationInfo.DelegatorUnbondingSig) // assert there is no delegator unbonding sig
		require.Equal(t, msg.BtcStaking.ActiveDel[0].BTCPkHex, consumerDelsResp.Delegations[0].BtcPkHex)
		require.Equal(t, msg.BtcStaking.ActiveDel[0].StartHeight, consumerDelsResp.Delegations[0].StartHeight)
		require.Equal(t, msg.BtcStaking.ActiveDel[0].EndHeight, consumerDelsResp.Delegations[0].EndHeight)
		require.Equal(t, msg.BtcStaking.ActiveDel[0].TotalSat, consumerDelsResp.Delegations[0].TotalSat)
		require.Equal(t, msg.BtcStaking.ActiveDel[0].StakingTx, base64.StdEncoding.EncodeToString(consumerDelsResp.Delegations[0].StakingTx))   // make sure to compare b64 encoded strings
		require.Equal(t, msg.BtcStaking.ActiveDel[0].SlashingTx, base64.StdEncoding.EncodeToString(consumerDelsResp.Delegations[0].SlashingTx)) // make sure to compare b64 encoded strings

		// ensure fp has voting power in smart contract
		consumerFpsByPowerResp, err := ctm.BcdConsumerClient.QueryFinalityProvidersByPower()
		require.NoError(t, err)
		require.NotNil(t, consumerFpsByPowerResp)
		require.Len(t, consumerFpsByPowerResp.Fps, 1)
		require.Equal(t, msg.BtcStaking.NewFP[0].BTCPKHex, consumerFpsByPowerResp.Fps[0].BtcPkHex)
		require.Equal(t, consumerDelsResp.Delegations[0].TotalSat, consumerFpsByPowerResp.Fps[0].Power)

		// inject pub rand commitment in smart contract (admin is not required, although in the tests admin and sender are the same)
		msg2 := GenPubRandomnessExecMsg(
			msgPub.FpBtcPk.MarshalHex(),
			base64.StdEncoding.EncodeToString(msgPub.Commitment),
			base64.StdEncoding.EncodeToString(msgPub.Sig.MustMarshal()),
			msgPub.StartHeight,
			msgPub.NumPubRand,
		)
		msgBytes2, err := json.Marshal(msg2)
		require.NoError(t, err)
		_, err = ctm.BcdConsumerClient.ExecuteContract(msgBytes2)
		require.NoError(t, err)

		// inject finality signature in smart contract (admin is not required, although in the tests admin and sender are the same)
		wasmdNodeStatus, err := ctm.BcdConsumerClient.GetCometNodeStatus()
		require.NoError(t, err)
		cometLatestHeight := wasmdNodeStatus.SyncInfo.LatestBlockHeight
		finalitySigMsg := GenFinalitySigExecMsg(uint64(1), uint64(cometLatestHeight), randList, fpSk)
		finalitySigMsgBytes, err := json.Marshal(finalitySigMsg)
		require.NoError(t, err)
		_, err = ctm.BcdConsumerClient.ExecuteContract(finalitySigMsgBytes)
		require.NoError(t, err)
		fpSigsResponse, err := ctm.BcdConsumerClient.QueryFinalitySignature(msgPub.FpBtcPk.MarshalHex(), uint64(cometLatestHeight))
		require.NoError(t, err)
		require.NotNil(t, fpSigsResponse)
		require.NotNil(t, fpSigsResponse.Signature)
		require.Equal(t, finalitySigMsg.SubmitFinalitySignature.Signature, base64.StdEncoding.EncodeToString(fpSigsResponse.Signature))
	*/
}
