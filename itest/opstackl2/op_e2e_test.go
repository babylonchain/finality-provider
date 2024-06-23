//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	e2etest "github.com/babylonchain/finality-provider/itest"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the btc staking contract using admin
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// store op-finality-gadget contract
	err := ctm.StoreWasmCode(opFinalityGadgetContractPath)
	require.NoError(t, err)
	opFinalityGadgetContractWasmId, err := ctm.GetLatestCodeId()
	require.NoError(t, err)
	require.Equal(t, uint64(1), opFinalityGadgetContractWasmId, "first deployed contract code_id should be 1")

	// instantiate op contract
	opFinalityGadgetInitMsg := map[string]interface{}{
		"admin":            ctm.OpL2ConsumerCtrl.CwClient.MustGetAddr(),
		"consumer_id":      opConsumerId,
		"activated_height": 0,
	}
	opFinalityGadgetInitMsgBytes, err := json.Marshal(opFinalityGadgetInitMsg)
	require.NoError(t, err)
	err = ctm.InstantiateWasmContract(opFinalityGadgetContractWasmId, opFinalityGadgetInitMsgBytes)
	require.NoError(t, err)

	// get op contract address
	resp, err := ctm.OpL2ConsumerCtrl.CwClient.ListContractsByCode(opFinalityGadgetContractWasmId, &sdkquerytypes.PageRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Contracts, 1)
	// update the contract address in config because during setup we had used a
	// mocked address which is diff than the deployed one on the fly
	ctm.OpL2ConsumerCtrl.Cfg.OPFinalityGadgetAddress = resp.Contracts[0]
	t.Logf("Deployed op finality contract address: %s", resp.Contracts[0])

	// register FP in Babylon Chain
	fpList := ctm.StartFinalityProvider(t, 1)

	// generate randomness data
	msgPub, err := ctm.GenerateCommitPubRandListMsg(fpList[0].GetBtcPkBIP340(), 1, 100)
	require.NoError(t, err)

	// inject pub rand commitment in smart contract
	commitPubRandMsg := e2etest.GenPubRandomnessExecMsg(
		msgPub.FpBtcPk.MarshalHex(),
		base64.StdEncoding.EncodeToString(msgPub.Commitment),
		base64.StdEncoding.EncodeToString(msgPub.Sig.MustMarshal()),
		msgPub.StartHeight,
		msgPub.NumPubRand,
	)
	commitPubRandMsgBytes, err := json.Marshal(commitPubRandMsg)
	require.NoError(t, err)

	err = ctm.ExecuteWasmContract(commitPubRandMsgBytes)
	require.NoError(t, err)

	// query pub rand
}
