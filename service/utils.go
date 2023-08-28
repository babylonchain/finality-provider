package service

import (
	"math/rand"
	"strings"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/types/errors"
)

var retriableErrors = []*errors.Error{
	errors.ErrInsufficientFunds,
	errors.ErrOutOfGas,
	errors.ErrInsufficientFee,
	errors.ErrMempoolIsFull,
}

// IsSubmissionErrRetriable returns true when the error is in the retriableErrors list
func IsSubmissionErrRetriable(err error) bool {
	for _, e := range retriableErrors {
		if strings.Contains(err.Error(), e.Error()) {
			return true
		}
	}

	return false
}

func GenerateRandPairList(num uint64) ([]*eots.PrivateRand, []types.SchnorrPubRand, error) {
	srList := make([]*eots.PrivateRand, num)
	prList := make([]types.SchnorrPubRand, num)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := uint64(0); i < num; i++ {
		eotsSR, eotsPR, err := eots.RandGen(r)
		if err != nil {
			return nil, nil, err
		}
		pr := types.NewSchnorrPubRandFromFieldVal(eotsPR)
		srList[i] = eotsSR
		prList[i] = *pr
	}
	return srList, prList, nil
}
