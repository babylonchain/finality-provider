package service

import (
	"sync"

	sdkmath "cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/babylonchain/finality-provider/types"
)

type createFinalityProviderResponse struct {
	FpInfo *proto.FinalityProviderInfo
}

type createFinalityProviderRequest struct {
	keyName         string
	passPhrase      string
	hdPath          string
	chainID         string
	description     *stakingtypes.Description
	commission      *sdkmath.LegacyDec
	errResponse     chan error
	successResponse chan *createFinalityProviderResponse
}

type registerFinalityProviderRequest struct {
	chainID   string
	bbnPubKey *secp256k1.PubKey
	btcPubKey *bbntypes.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	description     *stakingtypes.Description
	commission      *sdkmath.LegacyDec
	masterPubRand   string
	errResponse     chan error
	successResponse chan *RegisterFinalityProviderResponse
}

type finalityProviderRegisteredEvent struct {
	bbnPubKey       *secp256k1.PubKey
	btcPubKey       *bbntypes.BIP340PubKey
	txHash          string
	registeredEpoch uint64
	successResponse chan *RegisterFinalityProviderResponse
}

type RegisterFinalityProviderResponse struct {
	bbnPubKey       *secp256k1.PubKey
	btcPubKey       *bbntypes.BIP340PubKey
	TxHash          string
	RegisteredEpoch uint64
}

type CreateFinalityProviderResult struct {
	FpInfo *proto.FinalityProviderInfo
}

type fpState struct {
	mu sync.Mutex
	fp *store.StoredFinalityProvider
	s  *store.FinalityProviderStore
}

func (fps *fpState) getStoreFinalityProvider() *store.StoredFinalityProvider {
	fps.mu.Lock()
	defer fps.mu.Unlock()
	return fps.fp
}

func (fps *fpState) setStatus(s proto.FinalityProviderStatus) error {
	fps.mu.Lock()
	fps.fp.Status = s
	fps.mu.Unlock()
	return fps.s.SetFpStatus(fps.fp.BtcPk, s)
}

func (fps *fpState) setLastProcessedHeight(height uint64) error {
	fps.mu.Lock()
	fps.fp.LastProcessedHeight = height
	fps.mu.Unlock()
	return fps.s.SetFpLastProcessedHeight(fps.fp.BtcPk, height)
}

func (fps *fpState) setLastProcessedAndVotedHeight(height uint64) error {
	fps.mu.Lock()
	fps.fp.LastVotedHeight = height
	fps.fp.LastProcessedHeight = height
	fps.mu.Unlock()
	return fps.s.SetFpLastVotedHeight(fps.fp.BtcPk, height)
}

func (fp *FinalityProviderInstance) GetStoreFinalityProvider() *store.StoredFinalityProvider {
	return fp.state.getStoreFinalityProvider()
}

func (fp *FinalityProviderInstance) GetBtcPkBIP340() *bbntypes.BIP340PubKey {
	return fp.state.getStoreFinalityProvider().GetBIP340BTCPK()
}

func (fp *FinalityProviderInstance) GetBtcPk() *btcec.PublicKey {
	return fp.state.getStoreFinalityProvider().BtcPk
}

func (fp *FinalityProviderInstance) GetBtcPkHex() string {
	return fp.GetBtcPkBIP340().MarshalHex()
}

func (fp *FinalityProviderInstance) GetStatus() proto.FinalityProviderStatus {
	return fp.state.getStoreFinalityProvider().Status
}

func (fp *FinalityProviderInstance) GetLastVotedHeight() uint64 {
	return fp.state.getStoreFinalityProvider().LastVotedHeight
}

func (fp *FinalityProviderInstance) GetLastProcessedHeight() uint64 {
	return fp.state.getStoreFinalityProvider().LastProcessedHeight
}

func (fp *FinalityProviderInstance) GetChainID() []byte {
	return types.MarshalChainID(fp.state.getStoreFinalityProvider().ChainID)
}

func (fp *FinalityProviderInstance) SetStatus(s proto.FinalityProviderStatus) error {
	return fp.state.setStatus(s)
}

func (fp *FinalityProviderInstance) MustSetStatus(s proto.FinalityProviderStatus) {
	if err := fp.SetStatus(s); err != nil {
		fp.logger.Fatal("failed to set finality-provider status",
			zap.String("pk", fp.GetBtcPkHex()), zap.String("status", s.String()))
	}
}

func (fp *FinalityProviderInstance) SetLastProcessedHeight(height uint64) error {
	return fp.state.setLastProcessedHeight(height)
}

func (fp *FinalityProviderInstance) MustSetLastProcessedHeight(height uint64) {
	if err := fp.SetLastProcessedHeight(height); err != nil {
		fp.logger.Fatal("failed to set last processed height",
			zap.String("pk", fp.GetBtcPkHex()), zap.Uint64("last_processed_height", height))
	}
	fp.metrics.RecordFpLastProcessedHeight(fp.GetBtcPkHex(), height)
}

func (fp *FinalityProviderInstance) updateStateAfterFinalitySigSubmission(height uint64) error {
	return fp.state.setLastProcessedAndVotedHeight(height)
}

func (fp *FinalityProviderInstance) MustUpdateStateAfterFinalitySigSubmission(height uint64) {
	if err := fp.updateStateAfterFinalitySigSubmission(height); err != nil {
		fp.logger.Fatal("failed to update state after finality signature submitted",
			zap.String("pk", fp.GetBtcPkHex()), zap.Uint64("height", height))
	}
	fp.metrics.RecordFpLastVotedHeight(fp.GetBtcPkHex(), height)
	fp.metrics.RecordFpLastProcessedHeight(fp.GetBtcPkHex(), height)
}

func (fp *FinalityProviderInstance) getEOTSPrivKey() (*btcec.PrivateKey, error) {
	record, err := fp.em.KeyRecord(fp.btcPk.MustMarshal(), fp.passphrase)
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
}

// only used for testing purposes
func (fp *FinalityProviderInstance) BtcPrivKey() (*btcec.PrivateKey, error) {
	return fp.getEOTSPrivKey()
}

// only used for testing purposes
func (fp *FinalityProviderInstance) RegisteredEpoch() uint64 {
	return fp.state.fp.RegisteredEpoch
}
