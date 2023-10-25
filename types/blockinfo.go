package types

type BlockInfo struct {
	Height         uint64
	LastCommitHash []byte
	Finalized      bool
}
