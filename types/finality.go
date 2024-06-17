package types

import (
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
)

type CommitPublicRandomnessMsg struct {
	CommitPublicRandomness CommitPublicRandomnessMsgParams `json:"commit_public_randomness"`
}

type CommitPublicRandomnessMsgParams struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  []byte `json:"commitment"`
	Signature   []byte `json:"signature"`
}

// TODO: need to update based on contract implementation
type CommitPublicRandomnessResponse struct {
	Result bool `json:"result"`
}

type SubmitFinalitySignatureMsg struct {
	SubmitFinalitySignature SubmitFinalitySignatureMsgParams `json:"submit_finality_signature"`
}

type SubmitFinalitySignatureMsgParams struct {
	FpPubkeyHex string           `json:"fp_pubkey_hex"`
	Height      uint64           `json:"height"`
	PubRand     []byte           `json:"pub_rand"`
	Proof       *cmtcrypto.Proof `json:"proof"`
	BlockHash   []byte           `json:"block_hash"`
	Signature   []byte           `json:"signature"`
}

// TODO: need to update based on contract implementation
type SubmitFinalitySignatureResponse struct {
	Result bool `json:"result"`
}
