package e2etest

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/CosmWasm/wasmd/x/wasm/types"
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

// TestConsumerStoreContract stores a contract in the consumer chain
func TestConsumerStoreContract2(t *testing.T) {
	ctm, _ := StartConsumerManagerWithFps(t, 1)
	defer ctm.Stop(t)

	// Store the Babylon contract in the consumer chain
	babylonContractPath := "bytecode/babylon_contract.wasm"
	wasmCodeBytes := WasmCodeFileToBytes(t, babylonContractPath)

	storeMsg := &types.MsgStoreCode{
		Sender:       ctm.WasmdConsumerClient.WasmdClient.MustGetAddr(),
		WASMByteCode: wasmCodeBytes,
	}
	res, err := ctm.WasmdConsumerClient.ReliablySendMsg(storeMsg, nil, nil)
	require.NoError(t, err)
	fmt.Print(res)
	//latestCodeID, err := ctm.WasmdHandler.GetLatestCodeID()
	//require.NoError(t, err)
	//// Assert that the code id returned from store-code and list-code is the same
	//require.Equal(t, storedCodeID, latestCodeID)
	//
	//bb, _ := ctm.WasmdConsumerClient.QueryCometBestBlock()
	//fmt.Print(bb.Height)
}

func WasmCodeFileToBytes(t *testing.T, filename string) []byte {
	wasmCode, err := os.ReadFile(filename)
	require.NoError(t, err)
	if strings.HasSuffix(filename, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write(wasmCode)
		require.NoError(t, err)
		err = gz.Close()
		require.NoError(t, err)
		wasmCode = buf.Bytes()
	}
	return wasmCode
}

//// UnwrapExecTXResult is a helper to unpack execution result from proto any type
//func UnwrapExecTXResult(r *abci.ExecTxResult, target proto.Message) {
//	var wrappedRsp sdk.TxMsgData
//	require.NoError(chain.t, chain.Codec.Unmarshal(r.Data, &wrappedRsp))
//
//	// unmarshal protobuf response from data
//	require.Len(chain.t, wrappedRsp.MsgResponses, 1)
//	require.NoError(chain.t, proto.Unmarshal(wrappedRsp.MsgResponses[0].Value, target))
//}
