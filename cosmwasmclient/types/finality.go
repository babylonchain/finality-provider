package types

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
