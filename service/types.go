package service

import (
	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

type createValidatorResponse struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}
type createValidatorRequest struct {
	keyName         string
	errResponse     chan error
	successResponse chan *createValidatorResponse
}

type registerValidatorRequest struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *types.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	errResponse     chan error
	successResponse chan *registerValidatorResponse
}

type validatorRegisteredEvent struct {
	bbnPubKey       *secp256k1.PubKey
	txHash          []byte
	successResponse chan *registerValidatorResponse
}

type registerValidatorResponse struct {
	txHash []byte
}

type commitPubRandRequest struct {
	startingHeight uint64
	bbnPubKey      *secp256k1.PubKey
	valBtcPk       *types.BIP340PubKey
	privRandList   []*eots.PrivateRand
	pubRandList    []types.SchnorrPubRand
	sig            *types.BIP340Signature
}

type commitPubRandResponse struct {
	bbnPubKey    *secp256k1.PubKey
	privRandList []*eots.PrivateRand
	pubRandList  []types.SchnorrPubRand
	startHeight  uint64
	txHash       []byte
	err          error
}

type pubRandCommittedEvent struct {
	startingHeight  uint64
	bbnPubKey       *secp256k1.PubKey
	pubRandList     []types.SchnorrPubRand
	privRandList    []*eots.PrivateRand
	txHash          []byte
	successResponse chan struct{}
}

type addJurySigRequest struct {
	bbnPubKey       *secp256k1.PubKey
	valBtcPk        *types.BIP340PubKey
	delBtcPk        *types.BIP340PubKey
	sig             *types.BIP340Signature
	stakingTxHash   string
	errResponse     chan error
	successResponse chan *addJurySigResponse
}

type addJurySigResponse struct {
	txHash []byte
}

type jurySigAddedEvent struct {
	bbnPubKey       *secp256k1.PubKey
	txHash          []byte
	successResponse chan *addJurySigResponse
}

type addFinalitySigRequest struct {
	bbnPubKey           *secp256k1.PubKey
	valBtcPk            *types.BIP340PubKey
	blockHeight         uint64
	blockLastCommitHash []byte
	sig                 *types.SchnorrEOTSSig
}

type addFinalitySigResponse struct {
	bbnPubKey *secp256k1.PubKey
	height    uint64
	txHash    []byte
	err       error
}

type finalitySigAddedEvent struct {
	bbnPubKey       *secp256k1.PubKey
	height          uint64
	txHash          []byte
	successResponse chan struct{}
}

type CreateValidatorResult struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}
