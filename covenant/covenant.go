package covenant

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon/btcstaking"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	bbntypes "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/clientcontroller"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/val"
)

var (
	// TODO: Maybe configurable?
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

type CovenantEmulator struct {
	startOnce sync.Once
	stopOnce  sync.Once

	wg   sync.WaitGroup
	quit chan struct{}

	pk *btcec.PublicKey

	cc clientcontroller.ClientController
	kc *val.ChainKeyringController

	config *covcfg.Config
	params *types.StakingParams
	logger *logrus.Logger

	// input is used to pass passphrase to the keyring
	input      *strings.Reader
	passphrase string
}

func NewCovenantEmulator(
	config *covcfg.Config,
	cc clientcontroller.ClientController,
	passphrase string,
	logger *logrus.Logger,
) (*CovenantEmulator, error) {
	input := strings.NewReader("")
	kr, err := service.CreateKeyring(
		config.BabylonConfig.KeyDirectory,
		config.BabylonConfig.ChainID,
		config.BabylonConfig.KeyringBackend,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	kc, err := val.NewChainKeyringControllerWithKeyring(kr, config.BabylonConfig.Key, input)
	if err != nil {
		return nil, err
	}

	sk, err := kc.GetChainPrivKey(passphrase)
	if err != nil {
		return nil, fmt.Errorf("covenant key %s is not found: %w", config.BabylonConfig.Key, err)
	}

	pk, err := btcec.ParsePubKey(sk.PubKey().Bytes())
	if err != nil {
		return nil, err
	}

	return &CovenantEmulator{
		cc:         cc,
		kc:         kc,
		config:     config,
		logger:     logger,
		input:      input,
		passphrase: passphrase,
		pk:         pk,
		quit:       make(chan struct{}),
	}, nil
}

func (ce *CovenantEmulator) UpdateParams() error {
	params, err := ce.getParamsWithRetry()
	if err != nil {
		return err
	}
	ce.params = params

	return nil
}

// AddCovenantSignature adds a Covenant signature on the given Bitcoin delegation and submits it to Babylon
func (ce *CovenantEmulator) AddCovenantSignature(btcDel *types.Delegation) (*service.AddCovenantSigResponse, error) {
	// 1. the quorum is already achieved, skip sending more sigs
	if btcDel.HasCovenantQuorum(ce.params.CovenantQuorum) {
		return nil, nil
	}

	// 2. check staking tx and slashing tx are valid
	stakingMsgTx, _, err := bbntypes.NewBTCTxFromHex(btcDel.StakingTxHex)
	if err != nil {
		return nil, err
	}

	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(btcDel.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	slashingMsgTx, err := slashingTx.ToMsgTx()
	if err != nil {
		return nil, err
	}

	si, err := btcstaking.BuildStakingInfo(
		btcDel.BtcPk,
		btcDel.ValBtcPks,
		ce.params.CovenantPks,
		ce.params.CovenantQuorum,
		btcDel.GetStakingTime(),
		btcutil.Amount(btcDel.TotalSat),
		&ce.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	stakingOutputIdx, err := bbntypes.GetOutputIdxInBTCTx(stakingMsgTx, si.StakingOutput)
	if err != nil {
		return nil, err
	}

	if err := btcstaking.CheckTransactions(
		slashingMsgTx,
		stakingMsgTx,
		stakingOutputIdx,
		int64(ce.params.MinSlashingTxFeeSat),
		ce.params.SlashingRate,
		ce.params.SlashingAddress,
		&ce.config.ActiveNetParams,
	); err != nil {
		return nil, fmt.Errorf("invalid txs in the delegation: %w", err)
	}

	// 3. sign covenant sigs
	covenantPrivKey, err := ce.getPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Covenant private key: %w", err)
	}

	stakingInfo, err := btcstaking.BuildStakingInfo(
		btcDel.BtcPk,
		btcDel.ValBtcPks,
		ce.params.CovenantPks,
		ce.params.CovenantQuorum,
		btcDel.GetStakingTime(),
		btcutil.Amount(btcDel.TotalSat),
		&ce.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	slashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, err
	}

	covSigs := make([][]byte, 0, len(btcDel.ValBtcPks))
	for _, valPk := range btcDel.ValBtcPks {
		encKey, err := asig.NewEncryptionKeyFromBTCPK(valPk)
		if err != nil {
			return nil, err
		}
		covenantSig, err := slashingTx.EncSign(
			stakingMsgTx,
			btcDel.StakingOutputIdx,
			slashingPathInfo.GetPkScriptPath(),
			covenantPrivKey,
			encKey,
		)
		if err != nil {
			return nil, err
		}
		covSigs = append(covSigs, covenantSig.MustMarshal())
	}

	// 4. submit covenant sigs
	res, err := ce.cc.SubmitCovenantSigs(ce.pk, stakingMsgTx.TxHash().String(), covSigs)

	delPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(btcDel.BtcPk).MarshalHex()
	if err != nil {
		ce.logger.WithFields(logrus.Fields{
			"err":          err,
			"delegator_pk": delPkHex,
		}).Error("failed to submit Covenant signature")
		return nil, err
	}

	return &service.AddCovenantSigResponse{TxHash: res.TxHash}, nil
}

// AddCovenantUnbondingSignatures adds Covenant signature on the given Bitcoin delegation and submits it to Babylon
func (ce *CovenantEmulator) AddCovenantUnbondingSignatures(del *types.Delegation) (*service.AddCovenantSigResponse, error) {
	if del == nil {
		return nil, fmt.Errorf("btc delegation is nil")
	}

	if del.BtcUndelegation == nil {
		return nil, fmt.Errorf("delegation does not have an unbonding transaction")
	}

	// 1. skip sending sigs if the quorum has already been reached
	if del.BtcUndelegation.HasAllSignatures(ce.params.CovenantQuorum) {
		return nil, fmt.Errorf("undelegation of the delegation already has required signature from covenant")
	}

	// 2. ensure that the unbonding tx and the slashing tx are valid
	slashingMsgTx, _, err := bbntypes.NewBTCTxFromHex(del.BtcUndelegation.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	unbondingMsgTx, _, err := bbntypes.NewBTCTxFromHex(del.BtcUndelegation.UnbondingTxHex)
	if err != nil {
		return nil, err
	}

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		del.BtcPk,
		del.ValBtcPks,
		ce.params.CovenantPks,
		ce.params.CovenantQuorum,
		uint16(del.BtcUndelegation.UnbondingTime),
		btcutil.Amount(unbondingMsgTx.TxOut[0].Value),
		&ce.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	unbondingOutputIdx, err := bbntypes.GetOutputIdxInBTCTx(unbondingMsgTx, unbondingInfo.UnbondingOutput)
	if err != nil {
		return nil, err
	}

	stakingInfo, err := btcstaking.BuildStakingInfo(
		del.BtcPk,
		del.ValBtcPks,
		ce.params.CovenantPks,
		ce.params.CovenantQuorum,
		del.GetStakingTime(),
		btcutil.Amount(del.TotalSat),
		&ce.config.ActiveNetParams,
	)
	if err != nil {
		return nil, err
	}

	stakingTxUnbondigPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return nil, err
	}

	err = btcstaking.CheckTransactions(
		slashingMsgTx,
		unbondingMsgTx,
		unbondingOutputIdx,
		int64(ce.params.MinSlashingTxFeeSat),
		ce.params.SlashingRate,
		ce.params.SlashingAddress,
		&ce.config.ActiveNetParams,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid txs in the undelegation: %w", err)
	}

	// 3. Sign unbonding transaction
	covenantPrivKey, err := ce.getPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Covenant private key: %w", err)
	}
	stakingMsgTx, _, err := bbntypes.NewBTCTxFromHex(del.StakingTxHex)
	if err != nil {
		return nil, err
	}

	covenantUnbondingSignature, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingMsgTx,
		stakingMsgTx,
		del.StakingOutputIdx,
		stakingTxUnbondigPathInfo.GetPkScriptPath(),
		covenantPrivKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
	}

	// 4. Sign slash unbonding transaction
	slashUnbondingTx, err := bstypes.NewBTCSlashingTxFromHex(del.BtcUndelegation.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	unbondingTxSlashingPath, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, err
	}

	covSlashingSigs := make([][]byte, 0, len(del.ValBtcPks))
	for _, valPk := range del.ValBtcPks {
		encKey, err := asig.NewEncryptionKeyFromBTCPK(valPk)
		if err != nil {
			return nil, err
		}
		covenantSig, err := slashUnbondingTx.EncSign(
			unbondingMsgTx,
			0,
			unbondingTxSlashingPath.GetPkScriptPath(),
			covenantPrivKey,
			encKey,
		)
		if err != nil {
			return nil, err
		}
		covSlashingSigs = append(covSlashingSigs, covenantSig.MustMarshal())
	}

	// 5. submit covenant sigs
	res, err := ce.cc.SubmitCovenantUnbondingSigs(
		ce.pk,
		stakingMsgTx.TxHash().String(),
		covenantUnbondingSignature,
		covSlashingSigs,
	)

	delPkHex := bbntypes.NewBIP340PubKeyFromBTCPK(del.BtcPk).MarshalHex()

	if err != nil {
		ce.logger.WithFields(logrus.Fields{
			"err":          err,
			"delegator_pk": delPkHex,
		}).Error("failed to submit covenant signatures")
		return nil, err
	}

	return &service.AddCovenantSigResponse{
		TxHash: res.TxHash,
	}, nil
}

func (ce *CovenantEmulator) getPrivKey() (*btcec.PrivateKey, error) {
	sdkPrivKey, err := ce.kc.GetChainPrivKey(ce.passphrase)
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(sdkPrivKey.Key)

	return privKey, nil
}

// covenantSigSubmissionLoop is the reactor to submit Covenant signature for BTC delegations
func (ce *CovenantEmulator) covenantSigSubmissionLoop() {
	defer ce.wg.Done()

	interval := ce.config.QueryInterval
	limit := ce.config.DelegationLimit
	covenantSigTicker := time.NewTicker(interval)

	for {
		select {
		case <-covenantSigTicker.C:
			// 0. Update slashing address in case it is changed upon governance proposal
			if err := ce.UpdateParams(); err != nil {
				ce.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get staking params")
				continue
			}

			// 1. Get all pending delegations first, these are more important than the unbonding ones
			dels, err := ce.cc.QueryPendingDelegations(limit)
			if err != nil {
				ce.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get pending delegations")
				continue
			}
			if len(dels) == 0 {
				ce.logger.WithFields(logrus.Fields{}).Debug("no pending delegations are found")
			}

			for _, d := range dels {
				_, err := ce.AddCovenantSignature(d)
				if err != nil {
					ce.logger.WithFields(logrus.Fields{
						"err":        err,
						"del_btc_pk": d.BtcPk,
					}).Error("failed to submit Covenant sig to the Bitcoin delegation")
				}
			}

			// 2. Get all unbonding delegations
			unbondingDels, err := ce.cc.QueryUnbondingDelegations(limit)

			if err != nil {
				ce.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get pending delegations")
				continue
			}

			if len(unbondingDels) == 0 {
				ce.logger.WithFields(logrus.Fields{}).Debug("no unbonding delegations are found")
			}

			for _, d := range unbondingDels {
				_, err := ce.AddCovenantUnbondingSignatures(d)
				if err != nil {
					ce.logger.WithFields(logrus.Fields{
						"err":        err,
						"del_btc_pk": d.BtcPk,
					}).Error("failed to submit Covenant sig to the Bitcoin undelegation")
				}
			}

		case <-ce.quit:
			ce.logger.Debug("exiting covenant signature submission loop")
			return
		}
	}

}

func CreateCovenantKey(keyringDir, chainID, keyName, backend, passphrase, hdPath string) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	sdkCtx, err := service.CreateClientCtx(
		keyringDir, chainID,
	)
	if err != nil {
		return nil, nil, err
	}

	krController, err := val.NewChainKeyringController(
		sdkCtx,
		keyName,
		backend,
	)
	if err != nil {
		return nil, nil, err
	}

	sdkCovenantSk, _, err := krController.CreateChainKey(passphrase, hdPath)
	if err != nil {
		return nil, nil, err
	}

	covenantSk := secp256k1.PrivKeyFromBytes(sdkCovenantSk.Key)

	return covenantSk, covenantSk.PubKey(), nil
}

func (ce *CovenantEmulator) getParamsWithRetry() (*types.StakingParams, error) {
	var (
		params *types.StakingParams
		err    error
	)

	if err := retry.Do(func() error {
		params, err = ce.cc.QueryStakingParams()
		if err != nil {
			return err
		}
		return nil
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		ce.logger.WithFields(logrus.Fields{
			"attempt":      n + 1,
			"max_attempts": RtyAttNum,
			"error":        err,
		}).Debug("failed to query the consumer chain for the staking params")
	})); err != nil {
		return nil, err
	}

	return params, nil
}

func (ce *CovenantEmulator) Start() error {
	var startErr error
	ce.startOnce.Do(func() {
		ce.logger.Infof("Starting Covenant Emulator")

		ce.wg.Add(1)
		go ce.covenantSigSubmissionLoop()
	})

	return startErr
}

func (ce *CovenantEmulator) Stop() error {
	var stopErr error
	ce.stopOnce.Do(func() {
		ce.logger.Infof("Stopping Covenant Emulator")

		// Always stop the submission loop first to not generate additional events and actions
		ce.logger.Debug("Stopping submission loop")
		close(ce.quit)
		ce.wg.Wait()

		ce.logger.Debug("Covenant Emulator successfully stopped")
	})
	return stopErr
}
