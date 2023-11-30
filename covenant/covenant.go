package covenant

import (
	"fmt"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
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

// AddCovenantSignature adds a Covenant signature on the given Bitcoin delegation and submits it to Babylon
func (ce *CovenantEmulator) AddCovenantSignature(btcDel *types.Delegation) (*service.AddCovenantSigResponse, error) {
	// the quorum is already achieved, skip sending more sigs
	if btcDel.HasCovenantQuorum(ce.params.CovenantQuorum) {
		return nil, nil
	}

	stakingTx, _, err := bbntypes.NewBTCTxFromHex(btcDel.StakingTxHex)
	if err != nil {
		return nil, err
	}
	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(btcDel.SlashingTxHex)
	if err != nil {
		return nil, err
	}
	if err := slashingTx.Validate(
		&ce.config.ActiveNetParams,
		ce.params.SlashingAddress,
		sdkmath.LegacyNewDecFromBigInt(ce.params.SlashingRate),
		ce.params.MinSlashingTxFeeSat,
		stakingTx.TxOut[btcDel.StakingOutputIdx].Value,
	); err != nil {
		return nil, fmt.Errorf("invalid slashing tx in the delegation: %w", err)
	}

	// get Covenant private key from the keyring
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
			stakingTx,
			btcDel.StakingOutputIdx,
			slashingPathInfo.GetPkScriptPath(),
			covenantPrivKey,
			encKey,
		)
		covSigs = append(covSigs, covenantSig.MustMarshal())
	}

	res, err := ce.cc.SubmitCovenantSigs(ce.pk, stakingTx.TxHash().String(), covSigs)

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

	if del.BtcUndelegation.HasAllSignatures(ce.params.CovenantQuorum) {
		return nil, fmt.Errorf("undelegation of the delegation already has required signature from covenant")
	}

	// get Covenant private key from the keyring
	covenantPrivKey, err := ce.getPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get Covenant private key: %w", err)
	}

	// 1. Sign unbonding transaction
	stakingTx, _, err := bbntypes.NewBTCTxFromHex(del.StakingTxHex)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	unbondingTx, _, err := bbntypes.NewBTCTxFromHex(del.BtcUndelegation.UnbondingTxHex)
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

	covenantUnbondingSignature, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingTx,
		stakingTx,
		del.StakingOutputIdx,
		stakingTxUnbondigPathInfo.GetPkScriptPath(),
		covenantPrivKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
	}

	// 2. Sign slash unbonding transaction
	slashUnbondingTx, err := bstypes.NewBTCSlashingTxFromHex(del.BtcUndelegation.SlashingTxHex)
	if err != nil {
		return nil, err
	}
	if err := slashUnbondingTx.Validate(
		&ce.config.ActiveNetParams,
		ce.params.SlashingAddress,
		sdkmath.LegacyNewDecFromBigInt(ce.params.SlashingRate),
		ce.params.MinSlashingTxFeeSat,
		unbondingTx.TxOut[0].Value,
	); err != nil {
		return nil, fmt.Errorf("invalid slashing tx in the undelegation: %w", err)
	}

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		del.BtcPk,
		del.ValBtcPks,
		ce.params.CovenantPks,
		ce.params.CovenantQuorum,
		uint16(del.BtcUndelegation.UnbondingTime),
		btcutil.Amount(unbondingTx.TxOut[0].Value),
		&ce.config.ActiveNetParams,
	)
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
			unbondingTx,
			0,
			unbondingTxSlashingPath.GetPkScriptPath(),
			covenantPrivKey,
			encKey,
		)
		covSlashingSigs = append(covSlashingSigs, covenantSig.MustMarshal())
	}

	res, err := ce.cc.SubmitCovenantUnbondingSigs(
		ce.pk,
		stakingTx.TxHash().String(),
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
			params, err := ce.cc.QueryStakingParams()
			if err != nil {
				ce.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get staking params")
				continue
			}
			// update params
			ce.params = params

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
					}).Error("failed to submit Covenant sig to the Bitcoin delegation")
				}
			}

		case <-ce.quit:
			ce.logger.Debug("exiting covenant signature submission loop")
			return
		}
	}

}

func CreateCovenantKey(keyringDir, chainID, keyName, backend, passphrase, hdPath string) (*btcec.PublicKey, error) {
	sdkCtx, err := service.CreateClientCtx(
		keyringDir, chainID,
	)
	if err != nil {
		return nil, err
	}

	krController, err := val.NewChainKeyringController(
		sdkCtx,
		keyName,
		backend,
	)
	if err != nil {
		return nil, err
	}

	sdkCovenantPk, err := krController.CreateChainKey(passphrase, hdPath)
	if err != nil {
		return nil, err
	}

	covenantPk, err := secp256k1.ParsePubKey(sdkCovenantPk.Key)
	if err != nil {
		return nil, err
	}

	return covenantPk, nil
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
