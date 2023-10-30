package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	bbntypes "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

type ValidatorInstance struct {
	bbnPk *secp256k1.PubKey
	btcPk *bbntypes.BIP340PubKey

	state *valState
	cfg   *valcfg.Config

	logger *logrus.Logger
	em     eotsmanager.EOTSManager
	cc     clientcontroller.ClientController
	poller *ChainPoller

	laggingTargetChan chan *types.BlockInfo
	criticalErrChan   chan<- *CriticalError

	isStarted *atomic.Bool
	inSync    *atomic.Bool
	isLagging *atomic.Bool

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewValidatorInstance returns a ValidatorInstance instance with the given Babylon public key
// the validator should be registered before
func NewValidatorInstance(
	valPk *bbntypes.BIP340PubKey,
	cfg *valcfg.Config,
	s *val.ValidatorStore,
	cc clientcontroller.ClientController,
	em eotsmanager.EOTSManager,
	errChan chan<- *CriticalError,
	logger *logrus.Logger,
) (*ValidatorInstance, error) {
	v, err := s.GetStoreValidator(valPk.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to retrive the validator %s from DB: %w", v.GetBabylonPkHexString(), err)
	}

	// ensure the validator has been registered
	if v.Status < proto.ValidatorStatus_REGISTERED {
		return nil, fmt.Errorf("the validator %s has not been registered", v.KeyName)
	}

	return &ValidatorInstance{
		btcPk: v.MustGetBIP340BTCPK(),
		bbnPk: v.GetBabylonPK(),
		state: &valState{
			v: v,
			s: s,
		},
		cfg:             cfg,
		logger:          logger,
		isStarted:       atomic.NewBool(false),
		inSync:          atomic.NewBool(false),
		isLagging:       atomic.NewBool(false),
		criticalErrChan: errChan,
		em:              em,
		cc:              cc,
	}, nil
}

func (v *ValidatorInstance) Start() error {
	if v.isStarted.Swap(true) {
		return fmt.Errorf("the validator instance %s is already started", v.GetBtcPkHex())
	}

	v.logger.Infof("Starting thread handling validator %s", v.GetBtcPkHex())

	startHeight, err := v.bootstrap()
	if err != nil {
		return fmt.Errorf("failed to bootstrap the validator %s: %w", v.GetBtcPkHex(), err)
	}

	v.logger.Infof("the validator %s has been bootstrapped to %v", v.GetBtcPkHex(), startHeight)

	poller := NewChainPoller(v.logger, v.cfg.PollerConfig, v.cc)

	if err := poller.Start(startHeight + 1); err != nil {
		return fmt.Errorf("failed to start the poller: %w", err)
	}

	v.poller = poller

	v.laggingTargetChan = make(chan *types.BlockInfo, 1)

	v.quit = make(chan struct{})

	v.wg.Add(1)
	go v.finalitySigSubmissionLoop()
	v.wg.Add(1)
	go v.randomnessCommitmentLoop()
	v.wg.Add(1)
	go v.unbondindSigSubmissionLoop()
	v.wg.Add(1)
	go v.checkLaggingLoop()

	return nil
}

func (v *ValidatorInstance) bootstrap() (uint64, error) {
	latestBlock, err := v.getLatestBlockWithRetry()
	if err != nil {
		return 0, err
	}

	if v.checkLagging(latestBlock) {
		_, err := v.tryFastSync(latestBlock)
		if err != nil {
			return 0, err
		}
	}

	startHeight, err := v.getPollerStartingHeight()
	if err != nil {
		return 0, err
	}

	return startHeight, nil
}

func (v *ValidatorInstance) Stop() error {
	if !v.isStarted.Swap(false) {
		return fmt.Errorf("the validator %s has already stopped", v.GetBtcPkHex())
	}

	if err := v.poller.Stop(); err != nil {
		return fmt.Errorf("failed to stop the poller: %w", err)
	}

	v.logger.Infof("stopping thread handling validator %s", v.GetBtcPkHex())

	close(v.quit)
	v.wg.Wait()

	v.logger.Debugf("the thread handling validator %s is successfully stopped", v.GetBtcPkHex())

	return nil
}

func (v *ValidatorInstance) IsRunning() bool {
	return v.isStarted.Load()
}

func (v *ValidatorInstance) signUnbondingTransactions(
	privKey *btcec.PrivateKey,
	toSign []*types.Delegation) ([]unbondingTxSigData, error) {

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
			stakerPk:      bbntypes.NewBIP340PubKeyFromBTCPK(delegation.BtcPk),
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
			schnorrSig, err := sd.signature.ToBTCSig()
			if err != nil {
				res = append(res, unbondingTxSigSendResult{
					err:           err,
					stakingTxHash: sd.stakingTxHash,
				})
			}
			_, err = v.cc.SubmitValidatorUnbondingSig(
				v.MustGetBtcPk(),
				sd.stakerPk.MustToBTCPK(),
				sd.stakingTxHash,
				schnorrSig,
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
			delegationsNeedingSignatures, err := v.cc.QueryValidatorUnbondingDelegations(
				v.MustGetBtcPk(),
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

			validatorPrivKey, err := v.getEOTSPrivKey()

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

func (v *ValidatorInstance) finalitySigSubmissionLoop() {
	defer v.wg.Done()

	for {
		select {
		case b := <-v.poller.GetBlockInfoChan():
			v.logger.WithFields(logrus.Fields{
				"btc_pk_hex":   v.GetBtcPkHex(),
				"block_height": b.Height,
			}).Debug("the validator received a new block, start processing")
			if b.Height <= v.GetLastProcessedHeight() {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":            v.GetBtcPkHex(),
					"block_height":          b.Height,
					"last_processed_height": v.GetLastProcessedHeight(),
					"last_voted_height":     v.GetLastVotedHeight(),
				}).Debug("the block has been processed before, skip processing")
				continue
			}
			// use the copy of the block to avoid the impact to other receivers
			nextBlock := *b
			should, err := v.shouldSubmitFinalitySignature(&nextBlock)

			if err != nil {
				v.reportCriticalErr(err)
				continue
			}

			if !should {
				v.MustSetLastProcessedHeight(nextBlock.Height)
				continue
			}

			res, err := v.retrySubmitFinalitySignatureUntilBlockFinalized(&nextBlock)
			if err != nil {
				v.reportCriticalErr(err)
				continue
			}
			if res != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":   v.GetBtcPkHex(),
					"block_height": b.Height,
					"tx_hash":      res.TxHash,
				}).Info("successfully submitted a finality signature to the consumer chain")
			}
		case targetBlock := <-v.laggingTargetChan:
			res, err := v.tryFastSync(targetBlock)
			v.isLagging.Store(false)
			if err != nil {
				if errors.Is(err, types.ErrValidatorSlashed) {
					v.reportCriticalErr(err)
					continue
				}
				v.logger.WithFields(logrus.Fields{
					"err":        err,
					"btc_pk_hex": v.GetBtcPkHex(),
				}).Error("failed to sync up, will try again later")
				continue
			}
			// response might be nil if sync is not needed
			if res != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":            v.GetBtcPkHex(),
					"tx_hashes":             res.Responses,
					"synced_height":         res.SyncedHeight,
					"last_processed_height": res.LastProcessedHeight,
				}).Info("successfully synced to the latest block")

				// set the poller to fetch blocks that have not been processed
				v.poller.SetNextHeightAndClearBuffer(v.GetLastProcessedHeight() + 1)
			}
		case <-v.quit:
			v.logger.Info("the finality signature submission loop is closing")
			return
		}
	}
}

func (v *ValidatorInstance) randomnessCommitmentLoop() {
	defer v.wg.Done()

	commitRandTicker := time.NewTicker(v.cfg.RandomnessCommitInterval)
	defer commitRandTicker.Stop()

	for {
		select {
		case <-commitRandTicker.C:
			tipBlock, err := v.getLatestBlockWithRetry()
			if err != nil {
				v.reportCriticalErr(err)
				continue
			}
			txRes, err := v.retryCommitPubRandUntilBlockFinalized(tipBlock)
			if err != nil {
				v.reportCriticalErr(err)
				continue
			}
			if txRes != nil {
				v.logger.WithFields(logrus.Fields{
					"btc_pk_hex":            v.GetBtcPkHex(),
					"last_committed_height": v.GetLastCommittedHeight(),
					"tx_hash":               txRes.TxHash,
				}).Info("successfully committed public randomness to the consumer chain")
			}

		case <-v.quit:
			v.logger.Info("the randomness commitment loop is closing")
			return
		}
	}
}

func (v *ValidatorInstance) checkLaggingLoop() {
	defer v.wg.Done()

	if v.cfg.FastSyncInterval == 0 {
		v.logger.Info("the fast sync is disabled")
		return
	}

	fastSyncTicker := time.NewTicker(v.cfg.FastSyncInterval)
	defer fastSyncTicker.Stop()

	for {
		select {
		case <-fastSyncTicker.C:
			if v.isLagging.Load() {
				// we are in fast sync mode, skip do not do checks
				continue
			}

			latestBlock, err := v.getLatestBlockWithRetry()
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":        err,
					"btc_pk_hex": v.GetBtcPkHex(),
				}).Error("failed to get the latest block of the consumer chain")
				continue
			}

			if v.checkLagging(latestBlock) {
				v.isLagging.Store(true)
				v.laggingTargetChan <- latestBlock
			}
		case <-v.quit:
			v.logger.Debug("the fast sync loop is closing")
			return
		}
	}
}

func (v *ValidatorInstance) tryFastSync(targetBlock *types.BlockInfo) (*FastSyncResult, error) {
	if v.inSync.Load() {
		return nil, fmt.Errorf("the validator %s is already in sync", v.GetBtcPkHex())
	}

	if v.GetLastCommittedHeight() <= v.GetLastVotedHeight() {
		if err := v.SetLastProcessedHeight(targetBlock.Height); err != nil {
			return nil, err
		}
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   v.GetBtcPkHex(),
			"block_height": targetBlock.Height,
		}).Debug("insufficient public randomness, jumping to the latest block")
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
			"block_height": targetBlock.Height,
		}).Debug("no finalized blocks yet, no need to catch up")
		return nil, nil
	}

	lastFinalizedHeight := lastFinalizedBlocks[0].Height
	lastProcessedHeight := v.GetLastProcessedHeight()

	// get the startHeight from the maximum of the lastVotedHeight and
	// the lastFinalizedHeight plus 1
	var startHeight uint64
	if lastFinalizedHeight < lastProcessedHeight {
		startHeight = lastProcessedHeight + 1
	} else {
		startHeight = lastFinalizedHeight + 1
	}

	if startHeight > targetBlock.Height {
		return nil, fmt.Errorf("the start height %v should not be higher than the current block %v", startHeight, targetBlock.Height)
	}

	v.logger.Debug("the validator is entering fast sync")

	return v.FastSync(startHeight, targetBlock.Height)
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

	power, err := v.GetVotingPowerWithRetry(b.Height)
	if err != nil {
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

func (v *ValidatorInstance) reportCriticalErr(err error) {
	v.criticalErrChan <- &CriticalError{
		err:      err,
		valBtcPk: v.GetBtcPkBIP340(),
	}
}

// checkLagging returns true if the lasted voted height is behind by a configured gap
func (v *ValidatorInstance) checkLagging(currentBlock *types.BlockInfo) bool {
	return currentBlock.Height >= v.GetLastProcessedHeight()+v.cfg.FastSyncGap
}

// retrySubmitFinalitySignatureUntilBlockFinalized periodically tries to submit finality signature until success or the block is finalized
// error will be returned if maximum retries have been reached or the query to the consumer chain fails
func (v *ValidatorInstance) retrySubmitFinalitySignatureUntilBlockFinalized(targetBlock *types.BlockInfo) (*types.TxResponse, error) {
	var failedCycles uint64

	// we break the for loop if the block is finalized or the signature is successfully submitted
	// error will be returned if maximum retries have been reached or the query to the consumer chain fails
	for {
		// error will be returned if max retries have been reached
		res, err := v.SubmitFinalitySignature(targetBlock)
		if err != nil {
			if clientcontroller.IsUnrecoverable(err) {
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
			finalized, err := v.checkBlockFinalization(targetBlock.Height)
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

func (v *ValidatorInstance) checkBlockFinalization(height uint64) (bool, error) {
	b, err := v.cc.QueryBlock(height)
	if err != nil {
		return false, err
	}

	return b.Finalized, nil
}

// retryCommitPubRandUntilBlockFinalized periodically tries to commit public rand until success or the block is finalized
// error will be returned if maximum retries have been reached or the query to the consumer chain fails
func (v *ValidatorInstance) retryCommitPubRandUntilBlockFinalized(targetBlock *types.BlockInfo) (*types.TxResponse, error) {
	var failedCycles uint64

	// we break the for loop if the block is finalized or the public rand is successfully committed
	// error will be returned if maximum retries have been reached or the query to the consumer chain fails
	for {
		// error will be returned if max retries have been reached
		res, err := v.CommitPubRand(targetBlock)
		if err != nil {
			if clientcontroller.IsUnrecoverable(err) {
				return nil, err
			}
			v.logger.WithFields(logrus.Fields{
				"btc_val_pk":          v.GetBtcPkHex(),
				"currFailures":        failedCycles,
				"target_block_height": targetBlock.Height,
				"error":               err,
			}).Error("err committing public randomness to the consumer chain")

			failedCycles += 1
			if failedCycles > v.cfg.MaxSubmissionRetries {
				return nil, fmt.Errorf("reached max failed cycles with err: %w", err)
			}
		} else {
			// the public randomness has been successfully submitted
			return res, nil
		}
		select {
		case <-time.After(v.cfg.SubmissionRetryInterval):
			// periodically query the index block to be later checked whether it is Finalized
			finalized, err := v.checkBlockFinalization(targetBlock.Height)
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
func (v *ValidatorInstance) CommitPubRand(tipBlock *types.BlockInfo) (*types.TxResponse, error) {
	lastCommittedHeight := v.GetLastCommittedHeight()

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
	// NOTE: currently, calling this will create and save a list of randomness
	// in case of failure, randomness that has been created will be overwritten
	// for safety reason as the same randomness must not be used twice
	// TODO: should consider an implementation that deterministically create
	//  randomness without saving it
	pubRandList, err := v.createPubRandList(startHeight)
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
	schnorrSig, err := v.em.SignSchnorrSig(v.btcPk.MustMarshal(), hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign the Schnorr signature: %w", err)
	}

	pubRandByteList := make([]*btcec.FieldVal, 0, len(pubRandList))
	for _, r := range pubRandList {
		pubRandByteList = append(pubRandByteList, r.ToFieldVal())
	}
	res, err := v.cc.CommitPubRandList(v.MustGetBtcPk(), startHeight, pubRandByteList, schnorrSig)
	if err != nil {
		// TODO Add retry. check issue: https://github.com/babylonchain/btc-validator/issues/34
		return nil, fmt.Errorf("failed to commit public randomness to the consumer chain: %w", err)
	}

	newLastCommittedHeight := startHeight + uint64(len(pubRandList)-1)

	v.MustSetLastCommittedHeight(newLastCommittedHeight)

	return res, nil
}

func (v *ValidatorInstance) createPubRandList(startHeight uint64) ([]bbntypes.SchnorrPubRand, error) {
	pubRandList, err := v.em.CreateRandomnessPairList(
		v.btcPk.MustMarshal(),
		v.GetChainID(),
		startHeight,
		uint32(v.cfg.NumPubRand),
	)
	if err != nil {
		return nil, err
	}

	schnorrPubRandList := make([]bbntypes.SchnorrPubRand, 0, len(pubRandList))
	for _, pr := range pubRandList {
		schnorrPubRandList = append(schnorrPubRandList, *bbntypes.NewSchnorrPubRandFromFieldVal(pr))
	}

	return schnorrPubRandList, nil
}

// SubmitFinalitySignature builds and sends a finality signature over the given block to the consumer chain
func (v *ValidatorInstance) SubmitFinalitySignature(b *types.BlockInfo) (*types.TxResponse, error) {
	eotsSig, err := v.signEotsSig(b)
	if err != nil {
		return nil, err
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitFinalitySig(v.MustGetBtcPk(), b.Height, b.LastCommitHash, eotsSig.ToModNScalar())
	if err != nil {
		return nil, fmt.Errorf("failed to send finality signature to the consumer chain: %w", err)
	}

	// update DB
	v.MustUpdateStateAfterFinalitySigSubmission(b.Height)

	return res, nil
}

// SubmitBatchFinalitySignatures builds and sends a finality signature over the given block to the consumer chain
// NOTE: the input blocks should be in the ascending order of height
func (v *ValidatorInstance) SubmitBatchFinalitySignatures(blocks []*types.BlockInfo) (*types.TxResponse, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("should not submit batch finality signature with zero block")
	}

	sigs := make([]*btcec.ModNScalar, 0, len(blocks))
	for _, b := range blocks {
		eotsSig, err := v.signEotsSig(b)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, eotsSig.ToModNScalar())
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitBatchFinalitySigs(v.MustGetBtcPk(), blocks, sigs)
	if err != nil {
		return nil, fmt.Errorf("failed to send a batch of finality signatures to the consumer chain: %w", err)
	}

	// update DB
	highBlock := blocks[len(blocks)-1]
	v.MustUpdateStateAfterFinalitySigSubmission(highBlock.Height)

	return res, nil
}

func (v *ValidatorInstance) signEotsSig(b *types.BlockInfo) (*bbntypes.SchnorrEOTSSig, error) {
	// build proper finality signature request
	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            v.btcPk,
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := v.em.SignEOTS(v.btcPk.MustMarshal(), v.GetChainID(), msgToSign, b.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to sign EOTS: %w", err)
	}

	return bbntypes.NewSchnorrEOTSSigFromModNScalar(sig), nil
}

// TestSubmitFinalitySignatureAndExtractPrivKey is exposed for presentation/testing purpose to allow manual sending finality signature
// this API is the same as SubmitFinalitySignature except that we don't constraint the voting height and update status
// Note: this should not be used in the submission loop
func (v *ValidatorInstance) TestSubmitFinalitySignatureAndExtractPrivKey(b *types.BlockInfo) (*types.TxResponse, *btcec.PrivateKey, error) {
	// check last committed height
	if v.GetLastCommittedHeight() < b.Height {
		return nil, nil, fmt.Errorf("the validator's last committed height %v is lower than the current block height %v",
			v.GetLastCommittedHeight(), b.Height)
	}

	eotsSig, err := v.signEotsSig(b)
	if err != nil {
		return nil, nil, err
	}

	// send finality signature to the consumer chain
	res, err := v.cc.SubmitFinalitySig(v.MustGetBtcPk(), b.Height, b.LastCommitHash, eotsSig.ToModNScalar())
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

func (v *ValidatorInstance) getPollerStartingHeight() (uint64, error) {
	if !v.cfg.ValidatorModeConfig.AutoChainScanningMode {
		return v.cfg.ValidatorModeConfig.StaticChainScanningStartHeight, nil
	}

	// Set initial block to the maximum of
	//    - last processed height
	//    - the latest Babylon finalised height
	// The above is to ensure that:
	//
	//	(1) Any validator that is eligible to vote for a block,
	//	 doesn't miss submitting a vote for it.
	//	(2) The validators do not submit signatures for any already
	//	 finalised blocks.
	initialBlockToGet := v.GetLastProcessedHeight()
	latestFinalisedBlock, err := v.latestFinalizedBlocksWithRetry(1)
	if err != nil {
		return 0, err
	}
	if len(latestFinalisedBlock) != 0 {
		if latestFinalisedBlock[0].Height > initialBlockToGet {
			initialBlockToGet = latestFinalisedBlock[0].Height
		}
	}

	// ensure that initialBlockToGet is at least 1
	if initialBlockToGet == 0 {
		initialBlockToGet = 1
	}
	return initialBlockToGet, nil
}

func (v *ValidatorInstance) latestFinalizedBlocksWithRetry(count uint64) ([]*types.BlockInfo, error) {
	var response []*types.BlockInfo
	if err := retry.Do(func() error {
		latestFinalisedBlock, err := v.cc.QueryLatestFinalizedBlocks(count)
		if err != nil {
			return err
		}
		response = latestFinalisedBlock
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		v.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("Failed to query babylon for the latest finalised blocks")
	})); err != nil {
		return nil, err
	}
	return response, nil
}

func (v *ValidatorInstance) getLatestBlockWithRetry() (*types.BlockInfo, error) {
	var (
		latestBlock *types.BlockInfo
		err         error
	)

	if err := retry.Do(func() error {
		latestBlock, err = v.cc.QueryBestBlock()
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		v.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("failed to query the consumer chain for the latest block")
	})); err != nil {
		return nil, err
	}

	return latestBlock, nil
}

func (v *ValidatorInstance) GetVotingPowerWithRetry(height uint64) (uint64, error) {
	var (
		power uint64
		err   error
	)

	if err := retry.Do(func() error {
		power, err = v.cc.QueryValidatorVotingPower(v.MustGetBtcPk(), height)
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		v.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("failed to query the voting power")
	})); err != nil {
		return 0, err
	}

	return power, nil
}

func (v *ValidatorInstance) GetValidatorSlashedWithRetry() (bool, error) {
	var (
		slashed bool
		err     error
	)

	if err := retry.Do(func() error {
		slashed, err = v.cc.QueryValidatorSlashed(v.MustGetBtcPk())
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		v.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("failed to query the voting power")
	})); err != nil {
		return false, err
	}

	return slashed, nil
}
