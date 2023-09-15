package service

import (
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type createValidatorResponse struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}
type createValidatorRequest struct {
	keyName         string
	description     *stakingtypes.Description
	errResponse     chan error
	successResponse chan *createValidatorResponse
}

type registerValidatorRequest struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *types.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	description     *stakingtypes.Description
	errResponse     chan error
	successResponse chan *RegisterValidatorResponse
}

type validatorRegisteredEvent struct {
	bbnPubKey       *secp256k1.PubKey
	txHash          string
	successResponse chan *RegisterValidatorResponse
}

type RegisterValidatorResponse struct {
	TxHash string
}

type AddJurySigResponse struct {
	TxHash string
}

type CreateValidatorResult struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}

type unbondingTxSigData struct {
	stakerPk      *types.BIP340PubKey
	stakingTxHash string
	signature     *types.BIP340Signature
}

type unbondingTxSigSendResult struct {
	err           error
	stakingTxHash string
}
