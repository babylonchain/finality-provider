package val

import (
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/babylonchain/btc-validator/valrpc"
)

func NewValidator(babylonPk *secp256k1.PubKey, btcPk *types.BIP340PubKey, keyName string, pop *bstypes.ProofOfPossession) *valrpc.Validator {
	return &valrpc.Validator{
		// TODO change the validator to accept public key types directly
		KeyName:   keyName,
		BabylonPk: babylonPk.Bytes(),
		BtcPk:     btcPk.MustMarshal(),
		// TODO will use btcstaking types to avoid conversion
		Pop: &valrpc.ProofOfPossession{
			BabylonSig: pop.BabylonSig,
			BtcSig:     pop.BtcSig.MustMarshal(),
		},
		Status: valrpc.ValidatorStatus_VALIDATOR_STATUS_CREATED,
	}
}
