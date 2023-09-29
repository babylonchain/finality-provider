package types

import (
	"fmt"

	"github.com/babylonchain/btc-validator/proto"
)

const SchnorrRandomnessLength = 32

func NewSchnorrRandPair(privRand []byte, pubRand []byte) (*proto.SchnorrRandPair, error) {
	if len(privRand) != SchnorrRandomnessLength {
		return nil, fmt.Errorf("a private randomness should be %v bytes", SchnorrRandomnessLength)
	}

	if len(pubRand) != SchnorrRandomnessLength {
		return nil, fmt.Errorf("a public randomness should be %v bytes", SchnorrRandomnessLength)
	}

	return &proto.SchnorrRandPair{
		PubRand: pubRand,
		SecRand: privRand,
	}, nil
}
