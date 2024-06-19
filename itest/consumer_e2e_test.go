//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	fptypes "github.com/babylonchain/finality-provider/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

// TODO: currently fp app is not started in consumer manager, make a separate test which submits finality signature and public randomness automatically through fp daemon

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
		"admin": ctm.WasmdConsumerClient.MustGetValidatorAddress(),
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
		"admin":                           ctm.WasmdConsumerClient.MustGetValidatorAddress(),
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
	// update the contract address in config because during setup we had used a random address which is not valid
	ctm.WasmdConsumerClient.SetBtcStakingContractAddress(btcStakingContractAddr.String())

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fpSk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	randList, msgPub, err := GenCommitPubRandListMsg(r, fpSk, 1, 1000)
	require.NoError(t, err)

	// inject fp and delegation in smart contract using admin
	msg := GenBtcStakingExecMsg(msgPub.FpBtcPk.MarshalHex())
	msgBytes, err := json.Marshal(msg)
	require.NoError(t, err)
	_, err = ctm.WasmdConsumerClient.Exec(msgBytes)
	require.NoError(t, err)

	// query finality providers in smart contract
	dataFromContract, err := ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"finality_providers": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var consumerFps fptypes.ConsumerFpsResponse
	err = json.Unmarshal(dataFromContract.Data, &consumerFps)
	require.NoError(t, err)
	require.Len(t, consumerFps.ConsumerFps, 1)
	require.Equal(t, msg.BtcStaking.NewFP[0].ConsumerID, consumerFps.ConsumerFps[0].ConsumerId)
	require.Equal(t, msg.BtcStaking.NewFP[0].BTCPKHex, consumerFps.ConsumerFps[0].BtcPkHex)

	// query delegations in smart contract
	dataFromContract, err = ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"delegations": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var consumerDels fptypes.ConsumerDelegationsResponse
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

	// ensure fp has voting power in smart contract
	dataFromContract, err = ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"finality_providers_by_power": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var fpPower fptypes.ConsumerFpsByPowerResponse
	err = json.Unmarshal(dataFromContract.Data, &fpPower)
	require.NoError(t, err)
	require.Len(t, fpPower.Fps, 1)
	require.Equal(t, msg.BtcStaking.NewFP[0].BTCPKHex, fpPower.Fps[0].BtcPkHex)
	require.Equal(t, consumerDels.ConsumerDelegations[0].TotalSat, fpPower.Fps[0].Power)

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
	_, err = ctm.WasmdConsumerClient.Exec(msgBytes2)
	require.NoError(t, err)

	// inject finality signature in smart contract (admin is not required, although in the tests admin and sender are the same)
	wasmdNodeStatus, err := ctm.WasmdConsumerClient.GetCometNodeStatus()
	require.NoError(t, err)
	cometLatestHeight := wasmdNodeStatus.SyncInfo.LatestBlockHeight
	finalitySigMsg := GenFinalitySignExecMsg(uint64(1), uint64(cometLatestHeight), randList, fpSk)
	finalitySigMsgBytes, err := json.Marshal(finalitySigMsg)
	require.NoError(t, err)
	_, err = ctm.WasmdConsumerClient.Exec(finalitySigMsgBytes)
	require.NoError(t, err)
	finalitySigQuery := fmt.Sprintf(`{"finality_signature": {"btc_pk_hex": "%s", "height": %d}}`, msgPub.FpBtcPk.MarshalHex(), cometLatestHeight)
	dataFromContract, err = ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), finalitySigQuery)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var fpSigsResponse fptypes.FinalitySignatureResponse
	err = json.Unmarshal(dataFromContract.Data, &fpSigsResponse)
	require.NoError(t, err)
	require.NotNil(t, fpSigsResponse.Signature)
	require.Equal(t, finalitySigMsg.SubmitFinalitySignature.Signature, base64.StdEncoding.EncodeToString(fpSigsResponse.Signature))
}
