package types

import (
	"encoding/base64"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
)

type FinalitySigExecMsg struct {
	SubmitFinalitySignature SubmitFinalitySignature `json:"submit_finality_signature"`
}

type BtcStakingExecMsg struct {
	BtcStaking BtcStaking `json:"btc_staking"`
}

type PubRandomnessExecMsg struct {
	CommitPublicRandomness CommitPublicRandomness `json:"commit_public_randomness"`
}

func GenBtcStakingExecMsg(fpHex string) BtcStakingExecMsg {
	// generate random delegation and finality provider
	_, newDel := genRandomBtcDelegation()
	newFp := genRandomFinalityProvider()

	// replace fields so delegation and finality provider are linked
	newFp.BTCPKHex = fpHex
	newDel.FpBtcPkList = []string{fpHex}

	// create the BtcStakingExecMsg instance
	executeMessage := BtcStakingExecMsg{
		BtcStaking: BtcStaking{
			NewFP:       []NewFinalityProvider{newFp},
			ActiveDel:   []ActiveBtcDelegation{newDel},
			SlashedDel:  []SlashedBtcDelegation{},
			UnbondedDel: []UnbondedBtcDelegation{},
		},
	}

	return executeMessage
}

func GenPubRandomnessExecMsg(fpHex, commitment, sig string, startHeight, numPubRand uint64) PubRandomnessExecMsg {
	// create the PubRandomnessExecMsg instance
	executeMessage := PubRandomnessExecMsg{
		CommitPublicRandomness: CommitPublicRandomness{
			FPPubKeyHex: fpHex,
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig,
		},
	}

	return executeMessage
}

func GenFinalitySignExecMsg(startHeight, blockHeight uint64, randListInfo *datagen.RandListInfo, sk *btcec.PrivateKey) FinalitySigExecMsg {
	fmsg := genAddFinalitySig(startHeight, blockHeight, randListInfo, sk)

	// iterate proof.aunts and convert them to base64 encoded strings
	var aunts []string
	for _, aunt := range fmsg.Proof.Aunts {
		aunts = append(aunts, base64.StdEncoding.EncodeToString(aunt))
	}

	msg := FinalitySigExecMsg{
		SubmitFinalitySignature: SubmitFinalitySignature{
			FpPubkeyHex: fmsg.FpBtcPk.MarshalHex(),
			Height:      fmsg.BlockHeight,
			PubRand:     base64.StdEncoding.EncodeToString(fmsg.PubRand.MustMarshal()),
			Proof: Proof{
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
