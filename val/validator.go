package val

import (
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/babylonchain/btc-validator/proto"
)

func NewValidator(babylonPk *secp256k1.PubKey, btcPk *types.BIP340PubKey, keyName string, pop *bstypes.ProofOfPossession) *proto.Validator {
	return &proto.Validator{
		// TODO change the validator to accept public key types directly
		KeyName:   keyName,
		BabylonPk: babylonPk.Bytes(),
		BtcPk:     btcPk.MustMarshal(),
		// TODO will use btcstaking types to avoid conversion
		Pop: &proto.ProofOfPossession{
			BabylonSig: pop.BabylonSig,
			BtcSig:     pop.BtcSig.MustMarshal(),
		},
		Status: proto.ValidatorStatus_CREATED,
	}
}
