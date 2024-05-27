package service

import (
	"sync"

	"github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/merkle"
)

type pubRandState struct {
	sync.Mutex
	s *store.PubRandProofStore
}

func NewPubRandState(s *store.PubRandProofStore) *pubRandState {
	return &pubRandState{s: s}
}

func (st *pubRandState) AddPubRandProofList(
	pubRandList []*btcec.FieldVal,
	proofList []*merkle.Proof,
) error {
	st.Lock()
	defer st.Unlock()

	return st.s.AddPubRandProofList(pubRandList, proofList)
}

func (st *pubRandState) GetPubRandProof(pubRand *btcec.FieldVal) ([]byte, error) {
	st.Lock()
	defer st.Unlock()

	return st.s.GetPubRandProof(pubRand)
}

func (st *pubRandState) GetPubRandProofList(pubRandList []*btcec.FieldVal) ([][]byte, error) {
	st.Lock()
	defer st.Unlock()

	return st.s.GetPubRandProofList(pubRandList)
}
