package e2etest

import (
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	zctypes "github.com/babylonchain/babylon/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

type ConsumerFpsResponse struct {
	ConsumerFps []SingleConsumerFpResponse `json:"fps"`
}

// SingleConsumerFpResponse represents the finality provider data returned by the contract query.
// For more details, refer to the following links:
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/packages/apis/src/btc_staking_api.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/src/msg.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/schema/btc-staking.json
type SingleConsumerFpResponse struct {
	BtcPkHex             string `json:"btc_pk_hex"`
	SlashedBabylonHeight uint64 `json:"slashed_babylon_height"`
	SlashedBtcHeight     uint64 `json:"slashed_btc_height"`
	ConsumerId           string `json:"consumer_id"`
}

// TODO: uncomment after the fpd/eotsd is working for consumer
//     currently the queries are not implemented on contract so poller/fpd/eotsd are not working
//func TestConsumerFinalityProviderRegistration(t *testing.T) {
//	ctm, _ := StartConsumerManagerWithFps(t, 1)
//	defer ctm.Stop(t)
//
//	consumerChainID := "consumer-chain-test-1"
//	_, err := ctm.BBNClient.RegisterConsumerChain(consumerChainID, "Consumer chain 1 (test)", "Test Consumer Chain 1")
//	require.NoError(t, err)
//
//	ctm.CreateFinalityProvidersForChain(t, consumerChainID, 1)
//}

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
		"admin": ctm.WasmdConsumerClient.WasmdClient.MustGetAddr(),
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
		"admin":                           ctm.WasmdConsumerClient.WasmdClient.MustGetAddr(),
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	msg := GenIBCPacket(t, r)
	msgBytes, err := zctypes.ModuleCdc.MarshalJSON(msg)
	require.NoError(t, err)
	err = ctm.WasmdConsumerClient.Exec(btcStakingContractAddr, msgBytes)
	require.NoError(t, err)

	// query finality providers in smart contract
	dataFromContract, err := ctm.WasmdConsumerClient.QuerySmartContractState(btcStakingContractAddr.String(), `{"finality_providers": {}}`)
	require.NoError(t, err)
	require.NotNil(t, dataFromContract)
	var consumerFps ConsumerFpsResponse
	err = json.Unmarshal(dataFromContract.Data, &consumerFps)
	require.NoError(t, err)
	require.Len(t, consumerFps.ConsumerFps, 1)
	require.Equal(t, msg.Packet.(*zctypes.ZoneconciergePacketData_BtcStaking).BtcStaking.NewFp[0].ConsumerId, consumerFps.ConsumerFps[0].ConsumerId)
	require.Equal(t, msg.Packet.(*zctypes.ZoneconciergePacketData_BtcStaking).BtcStaking.NewFp[0].BtcPkHex, consumerFps.ConsumerFps[0].BtcPkHex)

	// submit finality signature to the btc staking contract using admin
	finalitySigMsg := GenFinalitySignatureMessage(msg.Packet.(*zctypes.ZoneconciergePacketData_BtcStaking).BtcStaking.NewFp[0].BtcPkHex)
	finalitySigMsgBytes, err := json.Marshal(finalitySigMsg)
	require.NoError(t, err)
	err = ctm.WasmdConsumerClient.Exec(btcStakingContractAddr, finalitySigMsgBytes)
	// TODO: insert delegation and pub randomness to fix the error
	require.Error(t, err)

}

func NewBTCStakingPacketData(packet *bstypes.BTCStakingIBCPacket) *zctypes.ZoneconciergePacketData {
	return &zctypes.ZoneconciergePacketData{
		Packet: &zctypes.ZoneconciergePacketData_BtcStaking{
			BtcStaking: packet,
		},
	}
}

func GenIBCPacket(t *testing.T, r *rand.Rand) *zctypes.ZoneconciergePacketData {
	// generate a finality provider
	fpBTCSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fpBabylonSK, _, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenRandomCustomFinalityProvider(r, fpBTCSK, fpBabylonSK, "consumer-id")
	require.NoError(t, err)

	packet := &bstypes.BTCStakingIBCPacket{
		NewFp: []*bstypes.NewFinalityProvider{
			// TODO: fill empty data
			&bstypes.NewFinalityProvider{
				// Description: fp.Description,
				Commission: fp.Commission.String(),
				// BabylonPk:  fp.BabylonPk,
				BtcPkHex: fp.BtcPk.MarshalHex(),
				// Pop:        fp.Pop,
				ConsumerId: fp.ConsumerId,
			},
		},
		ActiveDel:   []*bstypes.ActiveBTCDelegation{},
		SlashedDel:  []*bstypes.SlashedBTCDelegation{},
		UnbondedDel: []*bstypes.UnbondedBTCDelegation{},
	}
	return NewBTCStakingPacketData(packet)
}

type SubmitFinalitySignature struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     string `json:"pub_rand"`   // base64 encoded
	Proof       Proof  `json:"proof"`      // nested struct
	BlockHash   string `json:"block_hash"` // base64 encoded
	Signature   string `json:"signature"`  // base64 encoded
}

type ExecuteMsg struct {
	SubmitFinalitySignature *SubmitFinalitySignature `json:"submit_finality_signature"`
}

type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash string   `json:"leaf_hash"` // base64 encoded
	Aunts    []string `json:"aunts"`     // base64 encoded
}

// Generate a finality signature message with mock data
func GenFinalitySignatureMessage(fpBtcPkHex string) *ExecuteMsg {
	height := uint64(123456)
	pubRand := base64.StdEncoding.EncodeToString([]byte("mock_pub_rand"))
	leafHash := base64.StdEncoding.EncodeToString([]byte("mock_leaf_hash"))
	blockHash := base64.StdEncoding.EncodeToString([]byte("mock_block_hash"))
	signature := base64.StdEncoding.EncodeToString([]byte("mock_signature"))

	msg := ExecuteMsg{
		SubmitFinalitySignature: &SubmitFinalitySignature{
			FpPubkeyHex: fpBtcPkHex,
			Height:      height,
			PubRand:     pubRand,
			Proof: Proof{
				Total:    0,
				Index:    0,
				LeafHash: leafHash,
				Aunts:    []string{},
			},
			BlockHash: blockHash,
			Signature: signature,
		},
	}

	return &msg
}
