package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const instanceTerminatingMsg = "terminating the instance due to critical error"

type state struct {
	v *proto.StoreValidator
	s *val.ValidatorStore
}

type ValidatorInstance struct {
	bbnPk *secp256k1.PubKey
	btcPk *bbntypes.BIP340PubKey

	state *state
	cfg   *valcfg.Config

	blocksToVote chan *types.BlockInfo
	logger       *logrus.Logger
	kc           *val.KeyringController
	cc           clientcontroller.ClientController

	isStarted *atomic.Bool
	InSync    *atomic.Bool

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
	cc clientcontroller.ClientController,
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
	blocksToVote := make(chan *types.BlockInfo, 100)

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
		isStarted:    atomic.NewBool(false),
		InSync:       atomic.NewBool(false),
		kc:           kc,
		cc:           cc,
	}, nil
}

func (v *ValidatorInstance) GetStoreValidator() *proto.StoreValidator {
	return v.state.v
}

func (v *ValidatorInstance) getNextBlockChan() <-chan *types.BlockInfo {
	return v.blocksToVote
}

func (v *ValidatorInstance) receiveBlock(b *types.BlockInfo) {
	v.blocksToVote <- b
}

func (v *ValidatorInstance) GetBabylonPk() *secp256k1.PubKey {
	return v.bbnPk
}

func (v *ValidatorInstance) GetBabylonPkHex() string {
	return hex.EncodeToString(v.bbnPk.Key)
}

func (v *ValidatorInstance) GetBtcPkBIP340() *bbntypes.BIP340PubKey {
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
	if v.isStarted.Swap(true) {
		return fmt.Errorf("the validator instance %s is already started", v.GetBtcPkHex())
	}

	v.logger.Infof("Starting thread handling validator %s", v.GetBtcPkHex())

	v.quit = make(chan struct{})

	v.wg.Add(1)
	go v.submissionLoop()
	v.wg.Add(1)
	go v.unbondindSigSubmissionLoop()

	return nil
}

func (v *ValidatorInstance) Stop() error {
	v.isStarted.Store(false)

	v.logger.Infof("Stopping thread handling validator %s", v.GetBtcPkHex())

	close(v.quit)
	v.wg.Wait()

	v.logger.Debugf("The thread handling validator %s is successfully stopped", v.GetBtcPkHex())

	return nil
}

func (v *ValidatorInstance) signUnbondingTransactions(
	privKey *btcec.PrivateKey,
	toSign []*bstypes.BTCDelegation) ([]unbondingTxSigData, error) {

	var dataWithSignatures []unbondingTxSigData
	for _, delegation := range toSign {
		fundingTx, err := delegation.StakingTx.ToMsgTx()

		if err != nil {
			return nil, fmt.Errorf("failed to deserialize staking tx: %w", err)
		}

		fundingTxHash := fundingTx.TxHash().String()

		txToSign := delegation.BtcUndelegation.UnbondingTx

		sig, err := txToSign.Sign(
			fundingTx,
			delegation.StakingTx.Script,
			privKey,
			&v.cfg.ActiveNetParams,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
		}

		utd := unbondingTxSigData{
			stakerPk:      delegation.BtcPk,
			stakingTxHash: fundingTxHash,
			signature:     sig,
		}

		dataWithSignatures = append(dataWithSignatures, utd)
	}

	return dataWithSignatures, nil
}

func (v *ValidatorInstance) sendSignaturesForUnbondingTransactions(sigsToSend []unbondingTxSigData) []unbondingTxSigSendResult {
	var eg errgroup.Group
	var mu sync.Mutex
	var res []unbondingTxSigSendResult

	for _, sigData := range sigsToSend {
		sd := sigData
		eg.Go(func() error {
			_, err := v.cc.SubmitValidatorUnbondingSig(
				v.GetBtcPkBIP340(),
				sd.stakerPk,
				sd.stakingTxHash,
				sd.signature,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				res = append(res, unbondingTxSigSendResult{
					err:           err,
					stakingTxHash: sd.stakingTxHash,
				})
			} else {
				res = append(res, unbondingTxSigSendResult{
					err:           nil,
					stakingTxHash: sd.stakingTxHash,
				})
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		// this should not happen as we do not return errors from our sending
		v.logger.Fatalf("Failed to wait for signatures send")
	}

	return res
}

func (v *ValidatorInstance) unbondindSigSubmissionLoop() {
	defer v.wg.Done()

	sendUnbondingSigTicker := time.NewTicker(v.cfg.UnbondingSigSubmissionInterval)
	defer sendUnbondingSigTicker.Stop()

	for {
		select {
		case <-sendUnbondingSigTicker.C:
			delegationsNeedingSignatures, err := v.cc.QueryBTCValidatorUnbondingDelegations(
				v.GetBtcPkBIP340(),
				// TODO: parameterize the max number of delegations to be queried
				// it should not be to high to not take too long time to sign them
				10,
			)

			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
				}).Error("failed to query Babylon for BTC validator unbonding delegations")
				continue
			}

			if len(delegationsNeedingSignatures) == 0 {
				continue
			}

			v.logger.WithFields(logrus.Fields{
				"num_delegations": len(delegationsNeedingSignatures),
				"btc_pk_hex":      v.GetBtcPkHex(),
			}).Debug("Retrieved delegations which need unbonding signatures")

			validatorPrivKey, err := v.kc.GetBtcPrivKey()

			if err != nil {
				// Kill the app, if we can't recover our private key, then we have some bug
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
				}).Fatalf("failed to get validator private key")
			}

			signed, err := v.signUnbondingTransactions(validatorPrivKey, delegationsNeedingSignatures)

			if err != nil {
				// We received some malformed data from Babylon either there is some bug in babylon code
				// or we are on some malcious fork. Log it as error and continue.
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
				}).Errorf("failed to sign unbonding transactions")
				continue
			}

			sendResult := v.sendSignaturesForUnbondingTransactions(signed)

			for _, res := range sendResult {
				if res.err != nil {
					// Just log send errors, as if we failed to submit signaute, we will retry in next tick
					v.logger.WithFields(logrus.Fields{
						"err":            res.err,
						"babylon_pk_hex": v.GetBabylonPkHex(),
						"staking_tx":     res.stakingTxHash,
					}).Errorf("failed to send signature for unbonding transaction")
				} else {
					v.logger.WithFields(logrus.Fields{
						"babylon_pk_hex": v.GetBabylonPkHex(),
						"staking_tx":     res.stakingTxHash,
					}).Infof("successfully sent signature for unbonding transaction")
				}
			}

		case <-v.quit:
			v.logger.Debug("the unbonding sig submission loop is closing")
			return
		}
	}
}

func (v *ValidatorInstance) submissionLoop() {
	defer v.wg.Done()

	commitRandTicker := time.NewTicker(v.cfg.RandomnessCommitInterval)
	defer commitRandTicker.Stop()

	for {
		select {
		case b := <-v.getNextBlockChan():
			v.logger.WithFields(logrus.Fields{
				"btc_pk_hex":   v.GetBtcPkHex(),
				"block_height": b.Height,
			}).Debug("the validator received a new block, start processing")
			if v.InSync.Load() {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": b.Height,
				}).Debug("the validator is in fast sync, skip processing new blocks")
				continue
			}
			if b.Height <= v.GetLastVotedHeight() {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":        v.GetBtcPkHex(),
					"block_height":      b.Height,
					"last_voted_height": v.GetLastVotedHeight(),
				}).Debug("the block has been voted before, skip voting")
				continue
			}
			res, err := v.tryFastSync()
			if err != nil {
				if strings.Contains(err.Error(), bstypes.ErrBTCValAlreadySlashed.Error()) {
					v.logger.Infof("the validator %s is slashed, terminating the instance", v.GetBtcPkHex())
					return
				}
				v.logger.WithFields(logrus.Fields{
					"err":        err,
					"btc_pk_hex": v.GetBtcPkHex(),
				}).Error("failed to sync up")
				continue
			}
			// res might be nil if sync is not needed
			if res != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex": v.GetBtcPkHex(),
					"tx_hash":    res.TxHash,
				}).Info("successfully synced to the latest block")
				continue
			}
			// use the copy of the block to avoid the impact to other receivers
			nextBlock := *b
			should, err := v.shouldSubmitFinalitySignature(&nextBlock)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":          err,
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": nextBlock.Height,
				}).Warnf(instanceTerminatingMsg)
				return
			}
			if !should {
				continue
			}
			res, err = v.retrySubmitFinalitySignatureUntilBlockFinalized(&nextBlock)
			if err != nil {
				if strings.Contains(err.Error(), bstypes.ErrBTCValAlreadySlashed.Error()) {
					v.logger.Infof("the validator %s is slashed, terminating the instance", v.GetBtcPkHex())
					return
				}
				v.logger.WithFields(logrus.Fields{
					"err":          err,
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": nextBlock.Height,
				}).Warnf(instanceTerminatingMsg)
				return
			}
			if res != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": nextBlock.Height,
					"tx_hash":      res.TxHash,
				}).Info("successfully submitted a finality signature to the consumer chain")
			}

		case <-commitRandTicker.C:
			tipBlock, err := v.getLatestBlock()
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":        err,
					"btc_pk_hex": v.GetBtcPkHex(),
				}).Warnf(instanceTerminatingMsg)
				return
			}
			txRes, err := v.CommitPubRand(tipBlock)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":          err,
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": tipBlock,
				}).Warnf(instanceTerminatingMsg)
				return
			}
			if txRes != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":            v.GetBtcPkHex(),
					"last_committed_height": v.GetLastCommittedHeight(),
					"tx_hash":               txRes.TxHash,
				}).Info("successfully committed public randomness to the consumer chain")
			}
		case <-v.quit:
			v.logger.Info("the finality signature and public randomness submission loop is closing")
			return
		}
	}
}

func (v *ValidatorInstance) tryFastSync() (*provider.RelayerTxResponse, error) {
	if v.InSync.Load() {
		return nil, fmt.Errorf("the validator %s is already in sync", v.GetBtcPkHex())
	}

	latestBlock, err := v.getLatestBlock()
	if err != nil {
		return nil, err
	}

	if !v.isBehind(latestBlock) {
		return nil, nil
	}

	// get the last finalized height
	lastFinalizedBlocks, err := v.cc.QueryLatestFinalizedBlocks(1)
	if err != nil {
		return nil, err
	}
	if lastFinalizedBlocks == nil {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   v.GetBtcPkHex(),
			"block_height": latestBlock.Height,
		}).Debug("no finalized blocks yet, no need to catch up")
		return nil, nil
	}

	lastFinalizedHeight := lastFinalizedBlocks[0].Height
	lastVotedHeight := v.GetLastVotedHeight()

	// get the startHeight from the maximum of the lastVotedHeight and
	// the lastFinalizedHeight plus 1
	var startHeight uint64
	if lastFinalizedHeight < lastVotedHeight {
		startHeight = lastVotedHeight + 1
	} else {
		startHeight = lastFinalizedHeight + 1
	}

	if startHeight == latestBlock.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":     v.GetBtcPkHex(),
			"start_height":   startHeight,
			"current_height": latestBlock.Height,
		}).Debug("the start height is equal to the current block height, no need to catch up")
		return nil, nil
	}

	return v.FastSync(startHeight, latestBlock.Height)
}

// shouldSubmitFinalitySignature checks all the conditions that a finality should not be sent:
// 1. the validator does not have voting power on the given block
// 2. the last committed height is lower than the block height as this indicates the validator
// does not have the corresponding public randomness
// 3. the block height is lower than the last voted height as this indicates that the validator
// does not need to send finality signature over this block
func (v *ValidatorInstance) shouldSubmitFinalitySignature(b *types.BlockInfo) (bool, error) {
	// check last voted height
	if v.GetLastVotedHeight() >= b.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":        v.GetBtcPkHex(),
			"block_height":      b.Height,
			"last_voted_height": v.GetLastVotedHeight(),
		}).Debug("the block has been voted before, skip voting")
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

	// TODO: add retry here or within the query
	power, err := v.cc.QueryValidatorVotingPower(v.GetBtcPkBIP340(), b.Height)
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

	return true, nil
}

// isBehind returns true if the lasted voted height is at least two blocks behind of the current block
func (v *ValidatorInstance) isBehind(currentBlock *types.BlockInfo) bool {
	return currentBlock.Height >= v.GetLastVotedHeight()+2
}

// retrySubmitFinalitySignatureUntilBlockFinalized periodically tries to submit finality signature until success or the block is finalized
// error will be returned if maximum retires have been reached or the query to the consumer chain fails
func (v *ValidatorInstance) retrySubmitFinalitySignatureUntilBlockFinalized(targetBlock *types.BlockInfo) (*provider.RelayerTxResponse, error) {
	var failedCycles uint64

	// we break the for loop if the block is finalized or the signature is successfully submitted
	// error will be returned if maximum retries have been reached or the query to the consumer chain fails
	for {
		// error will be returned if max retries have been reached
		res, err := v.SubmitFinalitySignature(targetBlock)
		if err != nil {
			if !clientcontroller.IsRetriable(err) {
				return nil, err
			}
			v.logger.WithFields(logrus.Fields{
				"currFailures":        failedCycles,
				"target_block_height": targetBlock.Height,
				"error":               err,
			}).Error("err submitting finality signature to the consumer chain")

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
			finalized, err := v.cc.QueryBlockFinalization(targetBlock.Height)
			if err != nil {
				return nil, fmt.Errorf("failed to query block finalization at height %v: %w", targetBlock.Height, err)
			}
			if finalized {
				v.logger.WithFields(logrus.Fields{
					"btc_val_pk":   v.GetBtcPkHex(),
					"block_height": targetBlock.Height,
				}).Debug("the block is already finalized, skip submission")
				return nil, nil
			}

		case <-v.quit:
			v.logger.Debugf("the validator instance %s is closing", v.GetBtcPkHex())
			return nil, nil
		}
	}
}

// CommitPubRand generates a list of Schnorr rand pairs,
// commits the public randomness for the managed validators,
// and save the randomness pair to DB
func (v *ValidatorInstance) CommitPubRand(tipBlock *types.BlockInfo) (*provider.RelayerTxResponse, error) {
	lastCommittedHeight, err := v.cc.QueryHeightWithLastPubRand(v.btcPk)
	if err != nil {
		return nil, fmt.Errorf("failed to query the consumer chain for the last committed height: %w", err)
	}

	if v.GetLastCommittedHeight() != lastCommittedHeight {
		// for some reason number of random numbers locally does not match the chain node
		// log it and try to recover somehow
		return nil, fmt.Errorf("the local last committed height %v does not match the remote last committed height %v",
			v.GetLastCommittedHeight(), lastCommittedHeight)
	}

	var startHeight uint64
	if lastCommittedHeight == uint64(0) {
		// the validator has never submitted public rand before
		startHeight = tipBlock.Height + 1
		// should not use subtraction because they are in the type of uint64
	} else if lastCommittedHeight < v.cfg.MinRandHeightGap+tipBlock.Height {
		// we are running out of the randomness
		startHeight = lastCommittedHeight + 1
	} else {
		v.logger.WithFields(logrus.Fields{
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
	sig := bbntypes.NewBIP340SignatureFromBTCSig(schnorrSig)

	res, err := v.cc.CommitPubRandList(v.btcPk, startHeight, pubRandList, &sig)
	if err != nil {
		// TODO Add retry. check issue: https://github.com/babylonchain/btc-validator/issues/34
		return nil, fmt.Errorf("failed to commit public randomness to the consumer chain: %w", err)
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
				"err":        err,
				"btc_pk_hex": v.GetBtcPkHex(),
			}).Fatal("err while saving committed random pair to DB")
		}
	}

	newLastCommittedHeight := startHeight + uint64(len(pubRandList)-1)
	if err := v.SetLastCommittedHeight(newLastCommittedHeight); err != nil {
		v.logger.WithFields(logrus.Fields{
			"err":        err,
			"btc_pk_hex": v.GetBtcPkHex(),
		}).Fatal("err while saving last committed height to DB")
	}

	return res, nil
}

func (v *ValidatorInstance) updateStatusWithPower(power uint64) error {
	if power == 0 {
		if v.GetStatus() == proto.ValidatorStatus_ACTIVE {
			// the validator is slashed or unbonded from the consumer chain
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

// SubmitFinalitySignature builds and sends a finality signature over the given block to the consumer chain
func (v *ValidatorInstance) SubmitFinalitySignature(b *types.BlockInfo) (*provider.RelayerTxResponse, error) {
	eotsSig, err := v.signEotsSig(b)
	if err != nil {
		return nil, err
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitFinalitySig(v.GetBtcPkBIP340(), b.Height, b.LastCommitHash, eotsSig)
	if err != nil {
		return nil, fmt.Errorf("failed to send finality signature to the consumer chain: %w", err)
	}

	// update DB
	if err := v.SetLastVotedHeight(b.Height); err != nil {
		return nil, fmt.Errorf("failed to update last voted height to %v in DB: %w", b.Height, err)
	}

	return res, nil
}

// SubmitBatchFinalitySignatures builds and sends a finality signature over the given block to the consumer chain
// NOTE: the input blocks should be in the ascending order of height
func (v *ValidatorInstance) SubmitBatchFinalitySignatures(blocks []*types.BlockInfo) (*provider.RelayerTxResponse, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("should not submit batch finality signature with zero block")
	}

	sigs := make([]*bbntypes.SchnorrEOTSSig, 0, len(blocks))
	for _, b := range blocks {
		eotsSig, err := v.signEotsSig(b)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, eotsSig)
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitBatchFinalitySigs(v.GetBtcPkBIP340(), blocks, sigs)
	if err != nil {
		return nil, fmt.Errorf("failed to send a batch of finality signatures to the consumer chain: %w", err)
	}

	// update DB
	highBlock := blocks[len(blocks)-1]
	if err := v.SetLastVotedHeight(highBlock.Height); err != nil {
		return nil, fmt.Errorf("failed to update last voted height to %v in DB: %w", highBlock.Height, err)
	}

	return res, nil
}

func (v *ValidatorInstance) signEotsSig(b *types.BlockInfo) (*bbntypes.SchnorrEOTSSig, error) {
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

	return bbntypes.NewSchnorrEOTSSigFromModNScalar(sig), nil
}

// TestSubmitFinalitySignatureAndExtractPrivKey is exposed for presentation/testing purpose to allow manual sending finality signature
// this API is the same as SubmitFinalitySignature except that we don't constraint the voting height and update status
// Note: this should not be used in the submission loop
func (v *ValidatorInstance) TestSubmitFinalitySignatureAndExtractPrivKey(b *types.BlockInfo) (*provider.RelayerTxResponse, *btcec.PrivateKey, error) {
	btcPk := v.GetBtcPkBIP340()

	// check last committed height
	if v.GetLastCommittedHeight() < b.Height {
		return nil, nil, fmt.Errorf("the validator's last committed height %v is lower than the current block height %v",
			v.GetLastCommittedHeight(), b.Height)
	}

	// check voting power
	power, err := v.cc.QueryValidatorVotingPower(btcPk, b.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query the consumer chain for the validator's voting power: %w", err)
	}
	if power == 0 {
		if v.GetStatus() == proto.ValidatorStatus_ACTIVE {
			// the validator is slashed or unbonded from the consumer chain
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

	eotsSig, err := v.signEotsSig(b)
	if err != nil {
		return nil, nil, err
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitFinalitySig(v.GetBtcPkBIP340(), b.Height, b.LastCommitHash, eotsSig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send finality signature to the consumer chain: %w", err)
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

func (v *ValidatorInstance) getLatestBlock() (*types.BlockInfo, error) {
	res, err := v.cc.QueryBestHeader()
	if err != nil {
		return nil, err
	}
	return &types.BlockInfo{
		Height:         uint64(res.Header.Height),
		LastCommitHash: res.Header.LastCommitHash,
	}, nil
}
