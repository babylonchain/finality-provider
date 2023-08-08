package service

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

type State struct {
	v *proto.ValidatorStored
	s *val.ValidatorStore
}

type Validator struct {
	bbnPk *secp256k1.PubKey
	btcPk *types.BIP340PubKey

	state *State
	cfg   *valcfg.Config

	blocksToVote chan *BlockInfo
	logger       *logrus.Logger
	kc           *val.KeyringController
	bc           bbncli.BabylonClient

	startOnce sync.Once
	stopOnce  sync.Once

	// wg and quit are responsible for submissions go routines
	wg   sync.WaitGroup
	quit chan struct{}
}

// NewValidator returns a Validator instance with the given Babylon public key
// the validator should be registered before
func NewValidator(
	bbnPk *secp256k1.PubKey,
	cfg *valcfg.Config,
	s *val.ValidatorStore,
	kr keyring.Keyring,
	bc bbncli.BabylonClient,
	logger *logrus.Logger,
) (*Validator, error) {
	v, err := s.GetValidatorStored(bbnPk.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrive the validator %s from DB: %w", v.GetBabylonPkHexString(), err)
	}

	// ensure the validator has been registered
	if v.Status < proto.ValidatorStatus_REGISTERED {
		return nil, fmt.Errorf("the validator %s has not been registered", v.KeyName)
	}

	kc, err := val.NewKeyringControllerWithKeyring(kr, v.KeyName)
	if err != nil {
		return nil, err
	}

	// TODO load unvoted blocks from WAL and insert them into the channel
	blocksToVote := make(chan *BlockInfo, 0)

	return &Validator{
		bbnPk: bbnPk,
		btcPk: v.MustGetBIP340BTCPK(),
		state: &State{
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

func (v *Validator) GetBlockInfoChan() chan *BlockInfo {
	return v.blocksToVote
}

func (v *Validator) GetBabylonPk() *secp256k1.PubKey {
	return v.bbnPk
}

func (v *Validator) GetBabylonPkHex() string {
	return hex.EncodeToString(v.bbnPk.Key)
}

func (v *Validator) GetBtcPk() *types.BIP340PubKey {
	return v.btcPk
}

func (v *Validator) GetBtcPkHex() string {
	return v.btcPk.MarshalHex()
}

func (v *Validator) GetStatus() proto.ValidatorStatus {
	return v.state.v.Status
}

func (v *Validator) GetLastVotedHeight() uint64 {
	return v.state.v.LastVotedHeight
}

func (v *Validator) GetLastCommittedHeight() uint64 {
	return v.state.v.LastCommittedHeight
}

func (v *Validator) SetStatus(s proto.ValidatorStatus) error {
	v.state.v.Status = s
	return v.state.s.SaveValidator(v.state.v)
}

func (v *Validator) SetLastVotedHeight(height uint64) error {
	v.state.v.LastVotedHeight = height
	return v.state.s.SaveValidator(v.state.v)
}

func (v *Validator) SetLastCommittedHeight(height uint64) error {
	v.state.v.LastCommittedHeight = height
	return v.state.s.SaveValidator(v.state.v)
}

func (v *Validator) Start() error {
	var startErr error
	v.startOnce.Do(func() {
		v.logger.Infof("Starting thread handling validator %s", v.GetBabylonPkHex())

		v.wg.Add(1)
		go v.submissionLoop()
	})

	return startErr
}

func (v *Validator) Stop() error {
	var stopErr error
	v.stopOnce.Do(func() {
		v.logger.Infof("Stopping thread handling validator %s", v.GetBabylonPkHex())

		close(v.quit)
		v.wg.Wait()

		v.logger.Debugf("The thread handling validator %s is successfully stopped", v.GetBabylonPkHex())
	})
	return stopErr
}

func (v *Validator) submissionLoop() {
	defer v.wg.Done()

	commitRandTicker := time.NewTicker(v.cfg.RandomnessCommitInterval)

	for {
		select {
		case b := <-v.GetBlockInfoChan():
			txHash, err := v.SubmitFinalitySignature(b)
			if err != nil {
				// TODO Add retry here until the block is finalized. check issue: https://github.com/babylonchain/btc-validator/issues/34
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"block_height":   b.Height,
				}).Error("failed to submit finality signature to Babylon")
				continue
			}
			if txHash != nil {
				v.logger.WithFields(logrus.Fields{
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"btc_pk_hex":     v.GetBtcPkHex(),
					"block_height":   b.Height,
					"tx_hash":        txHash,
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
			txHash, err := v.commitPubRand(tipBlock)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"err":            err,
					"babylon_pk_hex": v.GetBabylonPkHex(),
					"block_height":   tipBlock.Height,
				}).Error("failed to commit public randomness")
				continue
			}
			if txHash != nil {
				v.logger.WithFields(logrus.Fields{
					"babylon_pk_hex":        v.GetBabylonPkHex(),
					"btc_pk_hex":            v.GetBtcPkHex(),
					"last_committed_height": v.GetLastCommittedHeight(),
					"tx_hash":               txHash,
				}).Info("successfully committed public randomness to Babylon")
			}
		case <-v.quit:
			v.logger.Debug("exiting submissionLoop")
			return
		}
	}
}

// commitPubRand generates a list of Schnorr rand pairs,
// commits the public randomness for the managed validators,
// and save the randomness pair to DB
func (v *Validator) commitPubRand(tipBlock *BlockInfo) ([]byte, error) {
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

	txHash, err := v.bc.CommitPubRandList(v.btcPk, startHeight, pubRandList, &sig)
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

	return txHash, nil
}

// SubmitFinalitySignature builds and sends a finality signature over the given block to Babylon
// the signature will not be sent if
// 1. the last committed height is lower than the block height as this indicates the validator
// does not have the corresponding public randomness
// 2. the block height is lower than the last voted height as this indicates that the validator
// does not need to send finality signature over this block
// 3. the validator does not have voting power on the given block
func (v *Validator) SubmitFinalitySignature(b *BlockInfo) ([]byte, error) {
	btcPk := v.GetBtcPk()

	// check last committed height
	if v.GetLastCommittedHeight() < b.Height {
		return nil, fmt.Errorf("the validator's last committed height %v is lower than the current block height %v",
			v.GetLastCommittedHeight(), b.Height)
	}

	// check last voted height
	if v.GetLastVotedHeight() >= b.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":        btcPk.MarshalHex(),
			"block_height":      b.Height,
			"last_voted_height": v.GetLastVotedHeight(),
		}).Debug("the block's height should be higher than the last voted height, skip voting")

		// TODO: this could happen if the Babylon node is in recovery
		//  need to double check this case in the future, but currently,
		//  we do not need to return an error as it does not affect finalization
		return nil, nil
	}

	// check voting power
	power, err := v.bc.QueryValidatorVotingPower(btcPk, b.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to query Babylon for the validator's voting power: %w", err)
	}
	if power == 0 {
		if v.GetStatus() == proto.ValidatorStatus_ACTIVE {
			// the validator is slashed or unbonded from Babylon side
			if err := v.SetStatus(proto.ValidatorStatus_INACTIVE); err != nil {
				return nil, fmt.Errorf("cannot set the validator status: %w", err)
			}
		}
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   btcPk.MarshalHex(),
			"block_height": b.Height,
		}).Debug("the validator's voting power is 0, skip voting")

		return nil, nil
	}

	// update the status
	if v.GetStatus() == proto.ValidatorStatus_REGISTERED || v.GetStatus() == proto.ValidatorStatus_INACTIVE {
		if err := v.SetStatus(proto.ValidatorStatus_ACTIVE); err != nil {
			return nil, fmt.Errorf("cannot set the validator status: %w", err)
		}
	}

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
	txHash, _, err := v.bc.SubmitFinalitySig(v.GetBtcPk(), b.Height, b.LastCommitHash, eotsSig)
	if err != nil {
		return nil, fmt.Errorf("failed to send finality signature to Babylon: %w", err)
	}

	// update DB
	if err := v.SetLastVotedHeight(b.Height); err != nil {
		return nil, fmt.Errorf("failed to update last voted height to %v in DB: %w", b.Height, err)
	}

	return txHash, nil
}

func (v *Validator) buildFinalitySigRequest(b *BlockInfo) (*addFinalitySigRequest, error) {
	privRand, err := v.getCommittedPrivPubRand(b.Height)
	if err != nil {
		return nil, err
	}

	btcPrivKey, err := v.kc.GetBtcPrivKey()
	if err != nil {
		return nil, err
	}

	msg := &ftypes.MsgAddFinalitySig{
		ValBtcPk:            v.btcPk,
		BlockHeight:         b.Height,
		BlockLastCommitHash: b.LastCommitHash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(btcPrivKey, privRand, msgToSign)
	if err != nil {
		return nil, err
	}
	eotsSig := types.NewSchnorrEOTSSigFromModNScalar(sig)

	return &addFinalitySigRequest{
		bbnPubKey:           v.bbnPk,
		valBtcPk:            v.btcPk,
		blockHeight:         b.Height,
		blockLastCommitHash: b.LastCommitHash,
		sig:                 eotsSig,
	}, nil
}

func (v *Validator) getCommittedPrivPubRand(height uint64) (*eots.PrivateRand, error) {
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

func (v *Validator) getTipBabylonBlock() (*BlockInfo, error) {
	header, err := v.bc.QueryBestHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to query Babylon for the tip block: %w", err)
	}

	return &BlockInfo{
		Height:         uint64(header.Header.Height),
		LastCommitHash: header.Header.LastCommitHash,
	}, nil
}
