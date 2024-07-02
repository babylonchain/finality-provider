package opstackl2

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
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     []byte `json:"pub_rand"`
	Proof       Proof  `json:"proof"`
	BlockHash   []byte `json:"block_hash"`
	Signature   []byte `json:"signature"`
}

// TODO: need to update based on contract implementation
type SubmitFinalitySignatureResponse struct {
	Result bool `json:"result"`
}

type QueryMsg struct {
	Config             *Config             `json:"config,omitempty"`
	LastPubRandCommit  *LastPubRandCommit  `json:"last_pub_rand_commit,omitempty"`
	PubRandCommit      *PubRandCommit      `json:"pub_rand_commit,omitempty"`
	FirstPubRandCommit *FirstPubRandCommit `json:"first_pub_rand_commit,omitempty"`
}

type Config struct{}

type LastPubRandCommit struct {
	BtcPkHex string `json:"btc_pk_hex"`
}

type PubRandCommit struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Height   uint64 `json:"height"`
}

type FirstPubRandCommit struct {
	BtcPkHex string `json:"btc_pk_hex"`
}

type ConfigResponse struct {
	ConsumerId      string `json:"consumer_id"`
	ActivatedHeight uint64 `json:"activated_height"`
}

type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash string   `json:"leaf_hash"`
	Aunts    []string `json:"aunts"`
}
