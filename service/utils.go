package service

import (
	"math/rand"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
)

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
