package service

import (
	"sync"

	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/val"
)

type createValidatorResponse struct {
	BtcValidatorPk     btcec.PublicKey
	BabylonValidatorPk secp256k1.PubKey
}
type createValidatorRequest struct {
	keyName         string
	description     *stakingtypes.Description
	commission      *sdktypes.Dec
	errResponse     chan error
	successResponse chan *createValidatorResponse
}

type registerValidatorRequest struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *bbntypes.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	description     *stakingtypes.Description
	commission      *sdktypes.Dec
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
	stakerPk      *bbntypes.BIP340PubKey
	stakingTxHash string
	signature     *bbntypes.BIP340Signature
}

type unbondingTxSigSendResult struct {
	err           error
	stakingTxHash string
}

type valState struct {
	mu sync.Mutex
	v  *proto.StoreValidator
	s  *val.ValidatorStore
}

func (vs *valState) getStoreValidator() *proto.StoreValidator {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return vs.v
}

func (vs *valState) setStatus(s proto.ValidatorStatus) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.v.Status = s
	return vs.s.UpdateValidator(vs.v)
}

func (vs *valState) setLastVotedHeight(height uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.v.LastVotedHeight = height
	return vs.s.UpdateValidator(vs.v)
}

func (vs *valState) setLastProcessedHeight(height uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.v.LastProcessedHeight = height
	return vs.s.UpdateValidator(vs.v)
}

func (vs *valState) setLastCommittedHeight(height uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.v.LastCommittedHeight = height
	return vs.s.UpdateValidator(vs.v)
}

func (vs *valState) setLastProcessedAndVotedHeight(height uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.v.LastVotedHeight = height
	vs.v.LastProcessedHeight = height
	return vs.s.UpdateValidator(vs.v)
}

func (v *ValidatorInstance) GetStoreValidator() *proto.StoreValidator {
	return v.state.getStoreValidator()
}

func (v *ValidatorInstance) GetBabylonPk() *secp256k1.PubKey {
	return v.state.getStoreValidator().GetBabylonPK()
}

func (v *ValidatorInstance) GetBabylonPkHex() string {
	return v.state.getStoreValidator().GetBabylonPkHexString()
}

func (v *ValidatorInstance) GetBtcPkBIP340() *bbntypes.BIP340PubKey {
	return v.state.getStoreValidator().MustGetBIP340BTCPK()
}

func (v *ValidatorInstance) MustGetBtcPk() *btcec.PublicKey {
	return v.state.getStoreValidator().MustGetBTCPK()
}

func (v *ValidatorInstance) GetBtcPkHex() string {
	return v.GetBtcPkBIP340().MarshalHex()
}

func (v *ValidatorInstance) GetStatus() proto.ValidatorStatus {
	return v.state.getStoreValidator().Status
}

func (v *ValidatorInstance) GetLastVotedHeight() uint64 {
	return v.state.getStoreValidator().LastVotedHeight
}

func (v *ValidatorInstance) GetLastProcessedHeight() uint64 {
	return v.state.getStoreValidator().LastProcessedHeight
}

func (v *ValidatorInstance) GetLastCommittedHeight() uint64 {
	return v.state.getStoreValidator().LastCommittedHeight
}

func (v *ValidatorInstance) SetStatus(s proto.ValidatorStatus) error {
	return v.state.setStatus(s)
}

func (v *ValidatorInstance) MustSetStatus(s proto.ValidatorStatus) {
	if err := v.SetStatus(s); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
			"status":     s.String(),
		}).Fatal("failed to set validator status")
	}
}

func (v *ValidatorInstance) SetLastVotedHeight(height uint64) error {
	return v.state.setLastVotedHeight(height)
}

func (v *ValidatorInstance) MustSetLastVotedHeight(height uint64) {
	if err := v.SetLastVotedHeight(height); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
			"height":     height,
		}).Fatal("failed to set last voted height")
	}
}

func (v *ValidatorInstance) SetLastProcessedHeight(height uint64) error {
	return v.state.setLastProcessedHeight(height)
}

func (v *ValidatorInstance) MustSetLastProcessedHeight(height uint64) {
	if err := v.SetLastProcessedHeight(height); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
			"height":     height,
		}).Fatal("failed to set last processed height")
	}
}

func (v *ValidatorInstance) SetLastCommittedHeight(height uint64) error {
	return v.state.setLastCommittedHeight(height)
}

func (v *ValidatorInstance) MustSetLastCommittedHeight(height uint64) {
	if err := v.SetLastCommittedHeight(height); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
			"height":     height,
		}).Fatal("failed to set last committed height")
	}
}

func (v *ValidatorInstance) updateStateAfterFinalitySigSubmission(height uint64) error {
	return v.state.setLastProcessedAndVotedHeight(height)
}

func (v *ValidatorInstance) MustUpdateStateAfterFinalitySigSubmission(height uint64) {
	if err := v.updateStateAfterFinalitySigSubmission(height); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
			"height":     height,
		}).Fatal("failed to update state after finality sig submission")
	}
}

// only used for testing purpose
func (v *ValidatorInstance) GetCommittedPubRandPairList() ([]*proto.SchnorrRandPair, error) {
	return v.state.s.GetRandPairList(v.bbnPk.Key)
}

// only used for testing purposes
func (v *ValidatorInstance) BtcPrivKey() (*btcec.PrivateKey, error) {
	return v.kc.GetBtcPrivKey()
}
