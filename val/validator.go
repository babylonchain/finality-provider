package val

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/protobuf/proto"

	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/valrpc"
)

func CreateValidator(babylonPk *btcec.PublicKey, btcPk *btcec.PublicKey) *valrpc.Validator {
	return &valrpc.Validator{
		BabylonPk:            babylonPk.SerializeCompressed(),
		BtcPk:                btcPk.SerializeCompressed(),
		LastVotedBlockHeight: 0,
		Status:               valrpc.ValidatorStatus_CREATED,
		CommittedRandList:    nil,
	}
}

func SaveValidator(s store.Store, val *valrpc.Validator) error {
	k := hex.EncodeToString(val.BabylonPk)
	v, err := proto.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal validator object: %w", err)
	}

	return s.Put(k, v)
}

func ListValidators(s store.Store) ([]*valrpc.Validator, error) {
	valsBytes, err := s.List("")
	if err != nil {
		return nil, err
	}

	valsList := make([]*valrpc.Validator, len(valsBytes))
	for i := 0; i < len(valsBytes); i++ {
		val := new(valrpc.Validator)
		err := proto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		valsList[i] = val
	}

	return valsList, nil
}
