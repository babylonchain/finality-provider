//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/json"
	"testing"

	"math/rand"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/finality-provider/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
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
	pubRandListInfo, msgPub, err := ctm.GenerateCommitPubRandListMsg(fpList[0].GetBtcPkBIP340(), 1, 100)
	require.NoError(t, err)

	// commit pub rand to smart contract
	commitRes, err := ctm.OpL2ConsumerCtrl.CommitPubRandList(
		msgPub.FpBtcPk.MustToBTCPK(),
		msgPub.StartHeight,
		msgPub.NumPubRand,
		msgPub.Commitment,
		msgPub.Sig.MustToBTCSig(),
	)
	require.NoError(t, err)
	t.Logf("Commit PubRandList to op finality contract %s", commitRes.TxHash)

	// query pub rand
	committedPubRandMap, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(msgPub.FpBtcPk.MustToBTCPK(), 1)
	require.NoError(t, err)
	for k, v := range committedPubRandMap {
		require.Equal(t, uint64(1), k)
		require.Equal(t, uint64(100), v.NumPubRand)
		require.Equal(t, msgPub.Commitment, v.Commitment)
		break
	}

	// mock block
	r := rand.New(rand.NewSource(1))
	block := &types.BlockInfo{
		Height: uint64(1),
		Hash:   datagen.GenRandomByteArray(r, 32),
	}
	// fp sign
	fpSig, err := fpList[0].SignFinalitySig(block)
	require.NoError(t, err)

	// pub rand proof
	proof, err := pubRandListInfo.ProofList[0].ToProto().Marshal()
	require.NoError(t, err)

	// submit finality signature to smart contract
	submitRes, err := ctm.OpL2ConsumerCtrl.SubmitFinalitySig(
		msgPub.FpBtcPk.MustToBTCPK(),
		block,
		pubRandListInfo.PubRandList[0],
		proof,
		fpSig.ToModNScalar(),
	)
	require.NoError(t, err)
	t.Logf("Submit finality signature to op finality contract %s", submitRes.TxHash)

	// mock more blocks
	blocks := []*types.BlockInfo{}
	var fpSigs []*secp256k1.ModNScalar
	for i := 2; i <= 4; i++ {
		block := &types.BlockInfo{
			Height: uint64(i),
			Hash:   datagen.GenRandomByteArray(r, 32),
		}
		blocks = append(blocks, block)
		// fp sign
		fpSig, err := fpList[0].SignFinalitySig(block)
		require.NoError(t, err)
		fpSigs = append(fpSigs, fpSig.ToModNScalar())
	}

	// proofs
	var proofs [][]byte
	for i := 1; i <= 3; i++ {
		proof, err := pubRandListInfo.ProofList[i].ToProto().Marshal()
		require.NoError(t, err)
		proofs = append(proofs, proof)
	}

	// submit batch finality signatures to smart contract
	batchSubmitRes, err := ctm.OpL2ConsumerCtrl.SubmitBatchFinalitySigs(
		msgPub.FpBtcPk.MustToBTCPK(),
		blocks,
		pubRandListInfo.PubRandList[1:4],
		proofs,
		fpSigs,
	)
	require.NoError(t, err)
	t.Logf("Submit batch finality signatures to op finality contract %s", batchSubmitRes.TxHash)
}
