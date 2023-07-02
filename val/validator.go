package val

import (
	"github.com/btcsuite/btcd/btcec/v2"

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
