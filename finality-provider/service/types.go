package service

import (
	"sync"

	sdkmath "cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/finality-provider/proto"
	fpstore "github.com/babylonchain/finality-provider/finality-provider/store"
)

type createFinalityProviderResponse struct {
	FpPk *bbntypes.BIP340PubKey
}
type createFinalityProviderRequest struct {
	keyName         string
	passPhrase      string
	hdPath          string
	chainID         string
	description     []byte
	commission      *sdkmath.LegacyDec
	errResponse     chan error
	successResponse chan *createFinalityProviderResponse
}

type registerFinalityProviderRequest struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *bbntypes.BIP340PubKey
	// TODO we should have our own representation of PoP
	pop             *btcstakingtypes.ProofOfPossession
	description     []byte
	commission      *sdkmath.LegacyDec
	errResponse     chan error
	successResponse chan *RegisterFinalityProviderResponse
}

type finalityProviderRegisteredEvent struct {
	bbnPubKey       *secp256k1.PubKey
	btcPubKey       *bbntypes.BIP340PubKey
	txHash          string
	successResponse chan *RegisterFinalityProviderResponse
}

type RegisterFinalityProviderResponse struct {
	bbnPubKey *secp256k1.PubKey
	btcPubKey *bbntypes.BIP340PubKey
	TxHash    string
}

type CreateFinalityProviderResult struct {
	FpPk *bbntypes.BIP340PubKey
}

type fpState struct {
	mu sync.Mutex
	fp *proto.StoreFinalityProvider
	s  *fpstore.FinalityProviderStore
}

func (fps *fpState) getStoreFinalityProvider() *proto.StoreFinalityProvider {
	fps.mu.Lock()
	defer fps.mu.Unlock()
	return fps.fp
}

func (fps *fpState) setStatus(s proto.FinalityProviderStatus) error {
	fps.mu.Lock()
	fps.fp.Status = s
	fps.mu.Unlock()
	return fps.s.UpdateFinalityProvider(fps.fp)
}

func (fps *fpState) setLastProcessedHeight(height uint64) error {
	fps.mu.Lock()
	fps.fp.LastProcessedHeight = height
	fps.mu.Unlock()
	return fps.s.UpdateFinalityProvider(fps.fp)
}

func (fps *fpState) setLastCommittedHeight(height uint64) error {
	fps.mu.Lock()
	fps.fp.LastCommittedHeight = height
	fps.mu.Unlock()
	return fps.s.UpdateFinalityProvider(fps.fp)
}

func (fps *fpState) setLastProcessedAndVotedHeight(height uint64) error {
	fps.mu.Lock()
	fps.fp.LastVotedHeight = height
	fps.fp.LastProcessedHeight = height
	fps.mu.Unlock()
	return fps.s.UpdateFinalityProvider(fps.fp)
}

func (fp *FinalityProviderInstance) GetStoreFinalityProvider() *proto.StoreFinalityProvider {
	return fp.state.getStoreFinalityProvider()
}

func (fp *FinalityProviderInstance) GetBabylonPk() *secp256k1.PubKey {
	return fp.state.getStoreFinalityProvider().GetBabylonPK()
}

func (fp *FinalityProviderInstance) GetBabylonPkHex() string {
	return fp.state.getStoreFinalityProvider().GetBabylonPkHexString()
}

func (fp *FinalityProviderInstance) GetBtcPkBIP340() *bbntypes.BIP340PubKey {
	return fp.state.getStoreFinalityProvider().MustGetBIP340BTCPK()
}

func (fp *FinalityProviderInstance) MustGetBtcPk() *btcec.PublicKey {
	return fp.state.getStoreFinalityProvider().MustGetBTCPK()
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

func (fp *FinalityProviderInstance) GetLastCommittedHeight() uint64 {
	return fp.state.getStoreFinalityProvider().LastCommittedHeight
}

func (fp *FinalityProviderInstance) GetChainID() []byte {
	return []byte(fp.state.getStoreFinalityProvider().ChainId)
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
}

func (fp *FinalityProviderInstance) SetLastCommittedHeight(height uint64) error {
	return fp.state.setLastCommittedHeight(height)
}

func (fp *FinalityProviderInstance) MustSetLastCommittedHeight(height uint64) {
	if err := fp.SetLastCommittedHeight(height); err != nil {
		fp.logger.Fatal("failed to set last committed height",
			zap.String("pk", fp.GetBtcPkHex()), zap.Uint64("last_committed_height", height))
	}
}

func (fp *FinalityProviderInstance) updateStateAfterFinalitySigSubmission(height uint64) error {
	return fp.state.setLastProcessedAndVotedHeight(height)
}

func (fp *FinalityProviderInstance) MustUpdateStateAfterFinalitySigSubmission(height uint64) {
	if err := fp.updateStateAfterFinalitySigSubmission(height); err != nil {
		fp.logger.Fatal("failed to update state after finality signature submitted",
			zap.String("pk", fp.GetBtcPkHex()), zap.Uint64("height", height))
	}
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
