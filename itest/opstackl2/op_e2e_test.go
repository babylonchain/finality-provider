//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	e2etest "github.com/babylonchain/finality-provider/itest"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

func storeWasmCode(opL2cc *opstackl2.OPStackL2ConsumerController, wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmdtypes.MsgStoreCode{
		Sender:       opL2cc.CwClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = opL2cc.ReliablySendMsg(storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func instantiateWasmContract(opL2cc *opstackl2.OPStackL2ConsumerController, codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: opL2cc.CwClient.MustGetAddr(),
		Admin:  opL2cc.CwClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "op-test",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := opL2cc.ReliablySendMsg(instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func executeWasmContract(opL2cc *opstackl2.OPStackL2ConsumerController, msgBytes []byte) error {
	execMsg := &wasmdtypes.MsgExecuteContract{
		Sender:   opL2cc.CwClient.MustGetAddr(),
		Contract: opL2cc.Cfg.OPFinalityGadgetAddress,
		Msg:      msgBytes,
	}

	_, err := opL2cc.ReliablySendMsg(execMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// returns the latest wasm code id.
func getLatestCodeId(opL2cc *opstackl2.OPStackL2ConsumerController) (uint64, error) {
	pagination := &sdkquerytypes.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := opL2cc.CwClient.ListCodes(pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}

// tests the finality signature submission to the btc staking contract using admin
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// store op-finality-gadget contract
	err := storeWasmCode(ctm.OpL2ConsumerCtrl, opFinalityGadgetContractPath)
	require.NoError(t, err)
	opFinalityGadgetContractWasmId, err := getLatestCodeId(ctm.OpL2ConsumerCtrl)
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
	err = instantiateWasmContract(ctm.OpL2ConsumerCtrl, opFinalityGadgetContractWasmId, opFinalityGadgetInitMsgBytes)
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
	/*
		Error:          Received unexpected error:
		rpc error: code = Unknown desc = rpc error: code = Unknown desc = failed to execute message; message index: 0:
		Finality provider not found for consumer op-stack-l2-12345 with pubkey cf694ae34e93b38563c585ffdb9b78f9a680d9b67e60e0cd93a660257aba3d26:
		execute wasm contract failed [CosmWasm/wasmd@v0.51.0/x/wasm/keeper/keeper.go:422] with gas used: '102297': unknown request
	*/
	// TODO:
	// - register FP to Babylon chain with the consumer id
	err = executeWasmContract(ctm.OpL2ConsumerCtrl, commitPubRandMsgBytes)
	require.NoError(t, err)

	// query pub rand
}
