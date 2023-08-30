package service

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

type state struct {
	v *proto.StoreValidator
	s *val.ValidatorStore
}

type ValidatorInstance struct {
	bbnPk *secp256k1.PubKey
	btcPk *types.BIP340PubKey

	state *state
	cfg   *valcfg.Config

	blocksToVote chan *BlockInfo
	logger       *logrus.Logger
	kc           *val.KeyringController
	bc           bbncli.BabylonClient

	startOnce sync.Once
	stopOnce  sync.Once

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewValidatorInstance returns a ValidatorInstance instance with the given Babylon public key
// the validator should be registered before
func NewValidatorInstance(
	bbnPk *secp256k1.PubKey,
	cfg *valcfg.Config,
	s *val.ValidatorStore,
	kr keyring.Keyring,
	bc bbncli.BabylonClient,
	logger *logrus.Logger,
) (*ValidatorInstance, error) {
	v, err := s.GetStoreValidator(bbnPk.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrive the validator %s from DB: %w", v.GetBabylonPkHexString(), err)
	}

	// ensure the validator has been registered
	// TODO refactor this by getting the constants from Babylon
	if v.Status < proto.ValidatorStatus_REGISTERED {
		return nil, fmt.Errorf("the validator %s has not been registered", v.KeyName)
	}

	kc, err := val.NewKeyringControllerWithKeyring(kr, v.KeyName)
	if err != nil {
		return nil, err
	}

	// TODO load unvoted blocks from WAL and insert them into the channel
	// TODO parameterize the buffer
	blocksToVote := make(chan *BlockInfo, 100)

	return &ValidatorInstance{
		bbnPk: bbnPk,
		btcPk: v.MustGetBIP340BTCPK(),
		state: &state{
			v: v,
			s: s,
		},
		cfg:          cfg,
		blocksToVote: blocksToVote,
		logger:       logger,
		kc:           kc,
		bc:           bc,
		quit:         make(chan struct{}),
	}, nil
}

func (v *ValidatorInstance) GetStoreValidator() *proto.StoreValidator {
	return v.state.v
}

func (v *ValidatorInstance) getNextBlockChan() <-chan *BlockInfo {
	return v.blocksToVote
}

func (v *ValidatorInstance) receiveBlock(b *BlockInfo) {
	v.blocksToVote <- b
}

func (v *ValidatorInstance) GetBabylonPk() *secp256k1.PubKey {
	return v.bbnPk
}

func (v *ValidatorInstance) GetBabylonPkHex() string {
	return hex.EncodeToString(v.bbnPk.Key)
}

func (v *ValidatorInstance) GetBtcPkBIP340() *types.BIP340PubKey {
	return v.btcPk
}

func (v *ValidatorInstance) MustGetBtcPk() *btcec.PublicKey {
	return v.btcPk.MustToBTCPK()
}

func (v *ValidatorInstance) GetBtcPkHex() string {
	return v.btcPk.MarshalHex()
}

func (v *ValidatorInstance) GetStatus() proto.ValidatorStatus {
	return v.state.v.Status
}

func (v *ValidatorInstance) GetLastVotedHeight() uint64 {
	return v.state.v.LastVotedHeight
}

func (v *ValidatorInstance) GetLastCommittedHeight() uint64 {
	return v.state.v.LastCommittedHeight
}

func (v *ValidatorInstance) GetCommittedPubRandPairList() ([]*proto.SchnorrRandPair, error) {
	return v.state.s.GetRandPairList(v.bbnPk.Key)
}

func (v *ValidatorInstance) GetCommittedPubRandPair(height uint64) (*proto.SchnorrRandPair, error) {
	return v.state.s.GetRandPair(v.bbnPk.Key, height)
}

func (v *ValidatorInstance) GetCommittedPrivPubRand(height uint64) (*eots.PrivateRand, error) {
	randPair, err := v.state.s.GetRandPair(v.bbnPk.Key, height)
	if err != nil {
		return nil, err
	}

	if len(randPair.SecRand) != 32 {
		return nil, fmt.Errorf("the private randomness should be 32 bytes")
	}

	privRand := new(eots.PrivateRand)
	privRand.SetByteSlice(randPair.SecRand)

	return privRand, nil
}

func (v *ValidatorInstance) SetStatus(s proto.ValidatorStatus) error {
	v.state.v.Status = s
	return v.state.s.UpdateValidator(v.state.v)
}

func (v *ValidatorInstance) SetLastVotedHeight(height uint64) error {
	v.state.v.LastVotedHeight = height
	return v.state.s.UpdateValidator(v.state.v)
}

func (v *ValidatorInstance) SetLastCommittedHeight(height uint64) error {
	v.state.v.LastCommittedHeight = height
	return v.state.s.UpdateValidator(v.state.v)
}

func (v *ValidatorInstance) Start() error {
	var startErr error
	v.startOnce.Do(func() {
		v.logger.Infof("Starting thread handling validator %s", v.GetBabylonPkHex())

		v.wg.Add(1)
		go v.submissionLoop()
	})

	return startErr
}

func (v *ValidatorInstance) Stop() error {
	var stopErr error
	v.stopOnce.Do(func() {
		v.logger.Infof("Stopping thread handling validator %s", v.GetBabylonPkHex())

		close(v.quit)
		v.wg.Wait()

		v.logger.Debugf("The thread handling validator %s is successfully stopped", v.GetBabylonPkHex())
	})
	return stopErr
}

func (v *ValidatorInstance) submissionLoop() {
	defer v.wg.Done()

	commitRandTicker := time.NewTicker(v.cfg.RandomnessCommitInterval)

	for {
		select {
		case b := <-v.getNextBlockChan():
			// use the copy of the block to avoid the impact to other receivers
			nextBlock := *b
			should, err := v.shouldSubmitFinalitySignature(&nextBlock)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":          err,
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": nextBlock.Height,
				}).Fatal("err when deciding if should send finality signature for the block")
			}
			if !should {
				continue
			}
			res, err := v.retrySubmitFinalitySignatureUntilBlockFinalized(&nextBlock)
			if err != nil {
				if strings.Contains(err.Error(), bstypes.ErrBTCValAlreadySlashed.Error()) {
					v.logger.Infof("the validator %s is slashed, terminating the instance", v.GetBabylonPkHex())
					return
				}
				v.logger.WithFields(logrus.Fields{
					"err":          err,
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": nextBlock.Height,
				}).Fatal("failed to submit finality signature to Babylon")
			}
			if res != nil {
				v.logger.WithFields(logrus.Fields{
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"btc_pk_hex":     v.GetBtcPkHex(),
					"block_height":   nextBlock.Height,
					"tx_hash":        res.TxHash,
				}).Info("successfully submitted a finality signature to Babylon")
			}

		case <-commitRandTicker.C:
			tipBlock, err := v.getTipBabylonBlock()
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
				}).Fatal("failed to get the current Babylon block")
			}
			txRes, err := v.CommitPubRand(tipBlock)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"block_height":   tipBlock,
				}).Fatal("failed to commit public randomness")
			}
			if txRes != nil {
				v.logger.WithFields(logrus.Fields{
					"babylon_pk_hex":        v.GetBabylonPkHex(),
					"btc_pk_hex":            v.GetBtcPkHex(),
					"last_committed_height": v.GetLastCommittedHeight(),
					"tx_hash":               txRes.TxHash,
				}).Info("successfully committed public randomness to Babylon")
			}
		case <-v.quit:
			v.logger.Infof("terminating the validator instance %s", v.GetBabylonPkHex())
			return
		}
	}
}

// shouldSubmitFinalitySignature checks all the conditions that a finality should not be sent:
// 1. the validator does not have voting power on the given block
// 2. the last committed height is lower than the block height as this indicates the validator
// does not have the corresponding public randomness
// 3. the block height is lower than the last voted height as this indicates that the validator
// does not need to send finality signature over this block
func (v *ValidatorInstance) shouldSubmitFinalitySignature(b *BlockInfo) (bool, error) {
	// TODO: add retry here or within the query
	power, err := v.bc.QueryValidatorVotingPower(v.GetBtcPkBIP340(), b.Height)
	if err != nil {
		return false, err
	}
	if err = v.updateStatusWithPower(power); err != nil {
		return false, err
	}
	if power == 0 {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   v.GetBtcPkHex(),
			"block_height": b.Height,
		}).Debug("the validator does not have voting power, skip voting")
		return false, nil
	}
	// check last committed height
	if v.GetLastCommittedHeight() < b.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":            v.GetBtcPkHex(),
			"last_committed_height": v.GetLastCommittedHeight(),
			"block_height":          b.Height,
		}).Debug("public rand is not committed, skip voting")
		return false, nil
	}

	// check last voted height
	if v.GetLastVotedHeight() >= b.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":        v.GetBtcPkHex(),
			"block_height":      b.Height,
			"last_voted_height": v.GetLastVotedHeight(),
		}).Debug("the block's height should be higher than the last voted height, skip voting")
		// TODO: this could happen if the Babylon node is in recovery
		//  need to double check this case in the future, but currently,
		//  we do not need to return an error as it does not affect finalization
		return false, nil
	}

	return true, err
}

// retrySubmitFinalitySignatureUntilBlockFinalized periodically tries to submit finality signature until success or the block is finalized
// error will be returned if maximum retires have been reached or the query to Babylon fails
func (v *ValidatorInstance) retrySubmitFinalitySignatureUntilBlockFinalized(targetBlock *BlockInfo) (*provider.RelayerTxResponse, error) {
	var failedCycles uint64

	// we break the for loop if the block is finalized or the signature is successfully submitted
	// error will be returned if maximum retries have been reached or the query to Babylon fails
	for {
		// error will be returned if max retries have been reached
		res, err := v.SubmitFinalitySignature(targetBlock)
		if err != nil {
			if !bbncli.IsRetriable(err) {
				return nil, err
			}
			v.logger.WithFields(logrus.Fields{
				"currFailures":        failedCycles,
				"target_block_height": targetBlock.Height,
				"error":               err,
			}).Error("err submitting finality signature to Babylon")

			failedCycles += 1
			if failedCycles > v.cfg.MaxSubmissionRetries {
				return nil, fmt.Errorf("reached max failed cycles with err: %w", err)
			}
		} else {
			// the signature has been successfully submitted
			return res, nil
		}
		select {
		case <-time.After(v.cfg.SubmissionRetryInterval):
			// periodically query the index block to be later checked whether it is Finalized
			ib, err := v.bc.QueryIndexedBlock(targetBlock.Height)
			if err != nil {
				return nil, fmt.Errorf("failed to query Babylon for block at height %v: %w", targetBlock.Height, err)
			}
			if ib.Height != targetBlock.Height || !bytes.Equal(ib.LastCommitHash, targetBlock.LastCommitHash) {
				// this means the chain is compromised
				return nil, fmt.Errorf("the queried block is not consistent with the target block, the chain is compromised")
			}
			if ib.Finalized {
				v.logger.WithFields(logrus.Fields{
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"block_height":   targetBlock.Height,
				}).Debug("the block is already finalized, skip submission")
				return nil, nil
			}

		case <-v.quit:
			v.logger.Debugf("the validator instance %s is closing", v.GetBabylonPkHex())
			return nil, nil
		}
	}
}

// CommitPubRand generates a list of Schnorr rand pairs,
// commits the public randomness for the managed validators,
// and save the randomness pair to DB
func (v *ValidatorInstance) CommitPubRand(tipBlock *BlockInfo) (*provider.RelayerTxResponse, error) {
	lastCommittedHeight, err := v.bc.QueryHeightWithLastPubRand(v.btcPk)
	if err != nil {
		return nil, fmt.Errorf("failed to query Babylon for the last committed height: %w", err)
	}

	if v.GetLastCommittedHeight() != lastCommittedHeight {
		// for some reason number of random numbers locally does not match babylon node
		// log it and try to recover somehow
		return nil, fmt.Errorf("the local last committed height %v does not match the remote last committed height %v",
			v.GetLastCommittedHeight(), lastCommittedHeight)
	}

	var startHeight uint64
	if lastCommittedHeight == uint64(0) {
		// the validator has never submitted public rand before
		startHeight = tipBlock.Height + 1
	} else if lastCommittedHeight-tipBlock.Height < v.cfg.MinRandHeightGap {
		// we are running out of the randomness
		startHeight = lastCommittedHeight + 1
	} else {
		v.logger.WithFields(logrus.Fields{
			"babylon_pk_hex":        v.GetBabylonPkHex(),
			"btc_pk_hex":            v.btcPk.MarshalHex(),
			"last_committed_height": v.GetLastCommittedHeight(),
			"current_block_height":  tipBlock.Height,
		}).Debug("the validator has sufficient public randomness, skip committing more")
		return nil, nil
	}

	// generate a list of Schnorr randomness pairs
	privRandList, pubRandList, err := GenerateRandPairList(v.cfg.NumPubRand)
	if err != nil {
		return nil, fmt.Errorf("failed to generate randomness: %w", err)
	}

	// get the message hash for signing
	msg := &ftypes.MsgCommitPubRandList{
		ValBtcPk:    v.btcPk,
		StartHeight: startHeight,
		PubRandList: pubRandList,
	}
	hash, err := msg.HashToSign()
	if err != nil {
		return nil, fmt.Errorf("failed to sign the commit public randomness message: %w", err)
	}

	// sign the message hash using the validator's BTC private key
	schnorrSig, err := v.kc.SchnorrSign(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign the Schnorr signature: %w", err)
	}
	sig := types.NewBIP340SignatureFromBTCSig(schnorrSig)

	res, err := v.bc.CommitPubRandList(v.btcPk, startHeight, pubRandList, &sig)
	if err != nil {
		// TODO Add retry. check issue: https://github.com/babylonchain/btc-validator/issues/34
		return nil, fmt.Errorf("failed to commit public randomness to Babylon: %w", err)
	}

	newLastCommittedHeight := startHeight + uint64(len(pubRandList)-1)
	if err := v.SetLastCommittedHeight(newLastCommittedHeight); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":            err,
			"babylon_pk_hex": v.GetBabylonPkHex(),
		}).Fatal("err while saving last committed height to DB")
	}

	// save the committed random list to DB
	// TODO 1: Optimize the db interface to batch the saving operations
	// TODO 2: Consider safety after recovery
	for i, pr := range privRandList {
		height := startHeight + uint64(i)
		privRand := pr.Bytes()
		randPair := &proto.SchnorrRandPair{
			SecRand: privRand[:],
			PubRand: pubRandList[i].MustMarshal(),
		}
		err = v.state.s.SaveRandPair(v.GetBabylonPk().Key, height, randPair)
		if err != nil {
			v.logger.WithFields(logrus.Fields{
				"err":            err,
				"babylon_pk_hex": v.GetBabylonPkHex(),
			}).Fatal("err while saving committed random pair to DB")
		}
	}

	return res, nil
}

func (v *ValidatorInstance) updateStatusWithPower(power uint64) error {
	if power == 0 {
		if v.GetStatus() == proto.ValidatorStatus_ACTIVE {
			// the validator is slashed or unbonded from Babylon side
			if err := v.SetStatus(proto.ValidatorStatus_INACTIVE); err != nil {
				return fmt.Errorf("cannot set the validator status: %w", err)
			}
		}

		return nil
	}

	// update the status
	if v.GetStatus() == proto.ValidatorStatus_REGISTERED || v.GetStatus() == proto.ValidatorStatus_INACTIVE {
		if err := v.SetStatus(proto.ValidatorStatus_ACTIVE); err != nil {
			return fmt.Errorf("cannot set the validator status: %w", err)
		}
	}

	return nil
}

// SubmitFinalitySignature builds and sends a finality signature over the given block to Babylon
func (v *ValidatorInstance) SubmitFinalitySignature(b *BlockInfo) (*provider.RelayerTxResponse, error) {
	// build proper finality signature request
	privRand, err := v.getCommittedPrivPubRand(b.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get the randomness pair from DB: %w", err)
	}
	btcPrivKey, err := v.kc.GetBtcPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC private key from the keyring: %w", err)
	}
	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            v.btcPk,
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(btcPrivKey, privRand, msgToSign)
	if err != nil {
		return nil, fmt.Errorf("failed to sign EOTS: %w", err)
	}
	eotsSig := types.NewSchnorrEOTSSigFromModNScalar(sig)

	// send finality signature to Babylon
	res, err := v.bc.SubmitFinalitySig(v.GetBtcPkBIP340(), b.Height, b.LastCommitHash, eotsSig)
	if err != nil {
		return nil, fmt.Errorf("failed to send finality signature to Babylon: %w", err)
	}

	// update DB
	if err := v.SetLastVotedHeight(b.Height); err != nil {
		return nil, fmt.Errorf("failed to update last voted height to %v in DB: %w", b.Height, err)
	}

	return res, nil
}

// TestSubmitFinalitySignatureAndExtractPrivKey is exposed for presentation/testing purpose to allow manual sending finality signature
// this API is the same as SubmitFinalitySignature except that we don't constraint the voting height and update status
// Note: this should not be used in the submission loop
func (v *ValidatorInstance) TestSubmitFinalitySignatureAndExtractPrivKey(b *BlockInfo) (*provider.RelayerTxResponse, *btcec.PrivateKey, error) {
	btcPk := v.GetBtcPkBIP340()

	// check last committed height
	if v.GetLastCommittedHeight() < b.Height {
		return nil, nil, fmt.Errorf("the validator's last committed height %v is lower than the current block height %v",
			v.GetLastCommittedHeight(), b.Height)
	}

	// check voting power
	power, err := v.bc.QueryValidatorVotingPower(btcPk, b.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query Babylon for the validator's voting power: %w", err)
	}
	if power == 0 {
		if v.GetStatus() == proto.ValidatorStatus_ACTIVE {
			// the validator is slashed or unbonded from Babylon side
			if err := v.SetStatus(proto.ValidatorStatus_INACTIVE); err != nil {
				return nil, nil, fmt.Errorf("cannot set the validator status: %w", err)
			}
		}
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   btcPk.MarshalHex(),
			"block_height": b.Height,
		}).Debug("the validator's voting power is 0, skip voting")

		return nil, nil, nil
	}

	// build proper finality signature request
	privRand, err := v.getCommittedPrivPubRand(b.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get the randomness pair from DB: %w", err)
	}
	btcPrivKey, err := v.kc.GetBtcPrivKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get BTC private key from the keyring: %w", err)
	}
	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            v.btcPk,
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(btcPrivKey, privRand, msgToSign)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign EOTS: %w", err)
	}
	eotsSig := types.NewSchnorrEOTSSigFromModNScalar(sig)

	// send finality signature to Babylon
	res, err := v.bc.SubmitFinalitySig(v.GetBtcPkBIP340(), b.Height, b.LastCommitHash, eotsSig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send finality signature to Babylon: %w", err)
	}

	// try to extract the private key
	var privKey *btcec.PrivateKey
	for _, ev := range res.Events {
		if strings.Contains(ev.EventType, "EventSlashedBTCValidator") {
			evidenceStr := ev.Attributes["evidence"]
			v.logger.Debugf("found slashing evidence %s", evidenceStr)
			var evidence ftypes.Evidence
			if err := jsonpb.UnmarshalString(evidenceStr, &evidence); err != nil {
				return nil, nil, fmt.Errorf("failed to decode evidence bytes to evidence: %s", err.Error())
			}
			privKey, err = evidence.ExtractBTCSK()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to extract private key: %s", err.Error())
			}
			break
		}
	}

	return res, privKey, nil
}

func (v *ValidatorInstance) getCommittedPrivPubRand(height uint64) (*eots.PrivateRand, error) {
	randPair, err := v.state.s.GetRandPair(v.bbnPk.Key, height)
	if err != nil {
		return nil, err
	}

	if len(randPair.SecRand) != 32 {
		return nil, fmt.Errorf("the private randomness should be 32 bytes")
	}

	privRand := new(eots.PrivateRand)
	privRand.SetByteSlice(randPair.SecRand)

	return privRand, nil
}

func (v *ValidatorInstance) getTipBabylonBlock() (*BlockInfo, error) {
	header, err := v.bc.QueryBestHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to query Babylon for the tip block: %w", err)
	}

	return &BlockInfo{
		Height:         uint64(header.Header.Height),
		LastCommitHash: header.Header.LastCommitHash,
	}, nil
}
