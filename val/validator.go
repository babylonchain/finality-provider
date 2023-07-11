package val

import (
	"github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/babylonchain/btc-validator/valrpc"
)

func NewValidator(babylonPk *secp256k1.PubKey, btcPk *types.BIP340PubKey) *valrpc.Validator {
	return &valrpc.Validator{
		// TODO change the validator to accept public key types directly
		BabylonPk: babylonPk.Bytes(),
		BtcPk:     btcPk.MustMarshal(),
		Status:    valrpc.ValidatorStatus_VALIDATOR_STATUS_CREATED,
	}
}
