package types

import (
	"encoding/base64"

	"github.com/babylonchain/babylon/testutil/datagen"
	fptypes "github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
)

func GenBtcStakingExecMsg(fpHex string) fptypes.BtcStakingExecMsg {
	// generate random delegation and finality provider
	_, newDel := genRandomBtcDelegation()
	newFp := genRandomFinalityProvider()

	// replace fields so delegation and finality provider are linked
	newFp.BTCPKHex = fpHex
	newDel.FpBtcPkList = []string{fpHex}

	// create the BtcStakingExecMsg instance
	executeMessage := fptypes.BtcStakingExecMsg{
		BtcStaking: fptypes.BtcStaking{
			NewFP:       []fptypes.NewFinalityProvider{newFp},
			ActiveDel:   []fptypes.ActiveBtcDelegation{newDel},
			SlashedDel:  []fptypes.SlashedBtcDelegation{},
			UnbondedDel: []fptypes.UnbondedBtcDelegation{},
		},
	}

	return executeMessage
}

func GenPubRandomnessExecMsg(fpHex, commitment, sig string, startHeight, numPubRand uint64) fptypes.PubRandomnessExecMsg {
	// create the PubRandomnessExecMsg instance
	executeMessage := fptypes.PubRandomnessExecMsg{
		CommitPublicRandomness: fptypes.CommitPublicRandomness{
			FPPubKeyHex: fpHex,
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig,
		},
	}

	return executeMessage
}

func GenFinalitySignExecMsg(startHeight, blockHeight uint64, randListInfo *datagen.RandListInfo, sk *btcec.PrivateKey) fptypes.FinalitySigExecMsg {
	fmsg := genAddFinalitySig(startHeight, blockHeight, randListInfo, sk)

	// iterate proof.aunts and convert them to base64 encoded strings
	var aunts []string
	for _, aunt := range fmsg.Proof.Aunts {
		aunts = append(aunts, base64.StdEncoding.EncodeToString(aunt))
	}

	msg := fptypes.FinalitySigExecMsg{
		SubmitFinalitySignature: fptypes.SubmitFinalitySignature{
			FpPubkeyHex: fmsg.FpBtcPk.MarshalHex(),
			Height:      fmsg.BlockHeight,
			PubRand:     base64.StdEncoding.EncodeToString(fmsg.PubRand.MustMarshal()),
			Proof: fptypes.Proof{
				Total:    uint64(fmsg.Proof.Total),
				Index:    uint64(fmsg.Proof.Index),
				LeafHash: base64.StdEncoding.EncodeToString(fmsg.Proof.LeafHash),
				Aunts:    aunts,
			},
			BlockHash: base64.StdEncoding.EncodeToString(fmsg.BlockAppHash),
			Signature: base64.StdEncoding.EncodeToString(fmsg.FinalitySig.MustMarshal()),
		},
	}

	return msg
}
