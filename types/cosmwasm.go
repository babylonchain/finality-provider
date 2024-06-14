package types

type SingleConsumerFpPowerResponse struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Power    uint64 `json:"power"`
}

type PubRandomnessExecMsg struct {
	CommitPublicRandomness CommitPublicRandomness `json:"commit_public_randomness"`
}

type CommitPublicRandomness struct {
	FPPubKeyHex string `json:"fp_pubkey_hex"`
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  string `json:"commitment"`
	Signature   string `json:"signature"`
}

type FinalitySigExecMsg struct {
	SubmitFinalitySignature SubmitFinalitySignature `json:"submit_finality_signature"`
}

type SubmitFinalitySignature struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     string `json:"pub_rand"`   // base64 encoded
	Proof       Proof  `json:"proof"`      // nested struct
	BlockHash   string `json:"block_hash"` // base64 encoded
	Signature   string `json:"signature"`  // base64 encoded
}

type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash string   `json:"leaf_hash"`
	Aunts    []string `json:"aunts"`
}
