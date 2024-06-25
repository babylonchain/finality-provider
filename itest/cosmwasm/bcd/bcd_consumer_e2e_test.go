//go:build e2e_bcd
// +build e2e_bcd

package e2etest_bcd

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

// TestConsumerFpLifecycle tests the consumer finality provider lifecycle
// 1. Upload Babylon and BTC staking contracts to bcd chain
// 2. Instantiate Babylon contract with admin
// 3. Register consumer chain to Babylon
// 4. Register finality provider to Babylon
// 5. Inject finality provider and delegation in BTC staking contract using admin
// 6. Start the finality provider daemon and app
// 7. Wait for fp daemon to submit public randomness and finality signature
func TestConsumerFpLifecycle(t *testing.T) {
	ctm := StartBcdTestManager(t)
	defer ctm.Stop(t)

	// store babylon contract
	babylonContractPath := "../../bytecode/babylon_contract.wasm"
	err := ctm.BcdConsumerClient.StoreWasmCode(babylonContractPath)
	require.NoError(t, err)
	babylonContractWasmId, err := ctm.BcdConsumerClient.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(1), babylonContractWasmId)

	// store btc staking contract
	btcStakingContractPath := "../../bytecode/btc_staking.wasm"
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

	// register consumer to babylon
	_, err = ctm.BBNClient.RegisterConsumerChain(bcdChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
	require.NoError(t, err)

	// register consumer fps to babylon
	app := ctm.Fpa
	cfg := app.GetConfig()
	fpName := itest.FpNamePrefix + bcdChainID
	moniker := itest.MonikerPrefix + bcdChainID
	commission := sdkmath.LegacyZeroDec()
	desc := itest.NewDescription(moniker)
	_, err = service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, bcdChainID, fpName, keyring.BackendTest, itest.Passphrase, itest.HdPath, "")
	require.NoError(t, err)
	res, err := app.CreateFinalityProvider(fpName, bcdChainID, itest.Passphrase, itest.HdPath, desc, &commission)
	require.NoError(t, err)
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
	require.NoError(t, err)
	_, err = app.RegisterFinalityProvider(fpPk.MarshalHex())
	require.NoError(t, err)

	// inject fp and delegation in smart contract using admin
	msg := itest.GenBtcStakingExecMsg(fpPk.MarshalHex())
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

	err = app.StartHandlingFinalityProvider(fpPk, itest.Passphrase)
	require.NoError(t, err)
	fpIns, err := app.GetFinalityProviderInstance(fpPk)
	require.NoError(t, err)
	require.True(t, fpIns.IsRunning())
	require.NoError(t, err)

	// ensure consumer finality providers are stored in Babylon
	require.Eventually(t, func() bool {
		fps, err := ctm.BBNClient.QueryConsumerFinalityProviders(bcdChainID)
		if err != nil {
			t.Logf("failed to query finality providers from Babylon %s", err.Error())
			return false
		}

		if len(fps) != 1 {
			return false
		}

		if !strings.Contains(fps[0].Description.Moniker, itest.MonikerPrefix) {
			return false
		}
		if !fps[0].Commission.Equal(sdkmath.LegacyZeroDec()) {
			return false
		}

		return true
	}, itest.EventuallyWaitTimeOut, itest.EventuallyPollTime)

	wasmdNodeStatus, err := ctm.BcdConsumerClient.GetCometNodeStatus()
	require.NoError(t, err)
	cometLatestHeight := wasmdNodeStatus.SyncInfo.LatestBlockHeight
	lookupHeight := cometLatestHeight + 20 // TODO: this is a hack, as its possible the randomness/sigs submission loops haven't started yet

	// ensure pub rand is submitted to smart contract
	require.Eventually(t, func() bool {
		fpPubRandResp, err := ctm.BcdConsumerClient.QueryLastCommittedPublicRand(fpPk.MustToBTCPK(), 1)
		if err != nil {
			t.Logf("failed to query last committed public rand: %s", err.Error())
			return false
		}
		if fpPubRandResp == nil {
			return false
		}

		return true
	}, itest.EventuallyWaitTimeOut, itest.EventuallyPollTime)

	// ensure finality signature is submitted to smart contract
	require.Eventually(t, func() bool {
		fpSigsResponse, err := ctm.BcdConsumerClient.QueryFinalitySignature(fpPk.MarshalHex(), uint64(lookupHeight))
		if err != nil {
			t.Logf("failed to query finality signature: %s", err.Error())
			return false
		}
		if fpSigsResponse == nil {
			return false
		}
		if fpSigsResponse.Signature == nil || len(fpSigsResponse.Signature) == 0 {
			return false
		}
		return true
	}, itest.EventuallyWaitTimeOut, itest.EventuallyPollTime)
}
