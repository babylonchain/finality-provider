package service

import (
	"encoding/hex"
	"time"

	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/proto"
)

// jurySigSubmissionLoop is the reactor to submit Jury signature for pending BTC delegations
func (app *ValidatorApp) jurySigSubmissionLoop() {
	defer app.wg.Done()

	interval := app.config.JuryModeConfig.QueryInterval
	limit := app.config.JuryModeConfig.DelegationLimit
	jurySigTicker := time.NewTicker(interval)

	for {
		select {
		case <-jurySigTicker.C:
			// 1. Get all pending delegations first, this are more important than the unbonding ones
			dels, err := app.cc.QueryBTCDelegations(btcstakingtypes.BTCDelegationStatus_PENDING, limit)
			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get pending delegations")
				continue
			}
			if len(dels) == 0 {
				app.logger.WithFields(logrus.Fields{}).Debug("no pending delegations are found")
			}

			for _, d := range dels {
				_, err := app.AddJurySignature(d)
				if err != nil {
					app.logger.WithFields(logrus.Fields{
						"err":        err,
						"del_btc_pk": d.BtcPk,
					}).Error("failed to submit Jury sig to the Bitcoin delegation")
				}
			}
			// 2. Get all unbonding delegations
			unbondingDels, err := app.cc.QueryBTCDelegations(btcstakingtypes.BTCDelegationStatus_UNBONDING, limit)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err": err,
				}).Error("failed to get pending delegations")
				continue
			}

			if len(unbondingDels) == 0 {
				app.logger.WithFields(logrus.Fields{}).Debug("no unbonding delegations are found")
			}

			for _, d := range unbondingDels {
				_, err := app.AddJuryUnbondingSignatures(d)
				if err != nil {
					app.logger.WithFields(logrus.Fields{
						"err":        err,
						"del_btc_pk": d.BtcPk,
					}).Error("failed to submit Jury sig to the Bitcoin delegation")
				}
			}

		case <-app.quit:
			app.logger.Debug("exiting jurySigSubmissionLoop")
			return
		}
	}

}

// main event loop for the validator app
func (app *ValidatorApp) eventLoop() {
	defer app.eventWg.Done()

	for {
		select {
		case req := <-app.createValidatorRequestChan:
			res, err := app.handleCreateValidatorRequest(req)
			if err != nil {
				req.errResponse <- err
				continue
			}

			req.successResponse <- &createValidatorResponse{ValPk: res.ValPk}

		case ev := <-app.validatorRegisteredEventChan:
			valStored, err := app.vs.GetStoreValidator(ev.btcPubKey.MustMarshal())

			if err != nil {
				// we always check if the validator is in the DB before sending the registration request
				app.logger.WithFields(logrus.Fields{
					"btc_pk":     ev.btcPubKey.MarshalHex(),
					"babylon_pk": hex.EncodeToString(ev.bbnPubKey.Key),
				}).Fatal("registered validator not found in DB")
			}

			// change the status of the validator to registered
			err = app.vs.SetValidatorStatus(valStored, proto.ValidatorStatus_REGISTERED)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"bbn_pk": ev.bbnPubKey,
				}).Fatal("err while saving validator to DB")
			}

			// return to the caller
			ev.successResponse <- &RegisterValidatorResponse{
				TxHash: ev.txHash,
			}

		case <-app.eventQuit:
			app.logger.Debug("exiting main eventLoop")
			return
		}
	}
}

func (app *ValidatorApp) registrationLoop() {
	defer app.sentWg.Done()
	for {
		select {
		case req := <-app.registerValidatorRequestChan:
			// we won't do any retries here to not block the loop for more important messages.
			// Most probably it fails due so some user error so we just return the error to the user.
			// TODO: need to start passing context here to be able to cancel the request in case of app quiting
			res, err := app.cc.RegisterValidator(req.bbnPubKey, req.btcPubKey, req.pop, req.commission, req.description)

			if err != nil {
				app.logger.WithFields(logrus.Fields{
					"err":        err,
					"btc_pk":     req.btcPubKey.MarshalHex(),
					"babylon_pk": hex.EncodeToString(req.bbnPubKey.Key),
				}).Error("failed to register validator")
				req.errResponse <- err
				continue
			}

			app.logger.WithFields(logrus.Fields{
				"btc_pk":     req.btcPubKey.MarshalHex(),
				"babylon_pk": hex.EncodeToString(req.bbnPubKey.Key),
				"txHash":     res.TxHash,
			}).Info("successfully registered validator on babylon")

			app.validatorRegisteredEventChan <- &validatorRegisteredEvent{
				btcPubKey: req.btcPubKey,
				bbnPubKey: req.bbnPubKey,
				txHash:    res.TxHash,
				// pass the channel to the event so that we can send the response to the user which requested
				// the registration
				successResponse: req.successResponse,
			}
		case <-app.sentQuit:
			app.logger.Debug("exiting sentToBabylonLoop")
			return
		}
	}
}
