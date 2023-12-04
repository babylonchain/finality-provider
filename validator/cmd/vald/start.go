package main

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"

	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/babylonchain/btc-validator/validator/service"
)

const (
	passphraseFlag = "passphrase"
	configFileFlag = "config"
	valPkFlag      = "validator-pk"

	defaultPassphrase = ""
)

var startValidator = cli.Command{
	Name:        "start",
	Usage:       "vald start",
	Description: "Start the validator daemon. Note that eotsd should be started beforehand",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The pass phrase used to decrypt the private key",
			Value: defaultPassphrase,
		},
		cli.StringFlag{
			Name:  configFileFlag,
			Usage: "The path to the covenant config file",
			Value: covcfg.DefaultConfigFile,
		},
		cli.StringFlag{
			Name:  valPkFlag,
			Usage: "The public key of the validator to start",
		},
	},
	Action: startValidatorFn,
}

func startValidatorFn(ctx *cli.Context) error {
	passphrase := ctx.String(passphraseFlag)
	configFilePath := ctx.String(configFileFlag)
	valPkStr := ctx.String(valPkFlag)

	cfg, cfgLogger, err := valcfg.LoadConfig(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	valApp, err := service.NewValidatorAppFromConfig(cfg, cfgLogger)
	if err != nil {
		return fmt.Errorf("failed to create validator app: %v", err)
	}

	// only start the daemon without starting any validator instance
	// as there might be no validator registered yet
	if err := valApp.Start(); err != nil {
		return fmt.Errorf("failed to start the validator daemon: %w", err)
	}

	if valPkStr != "" {
		// start the validator instance with the given public key
		valPk, err := types.NewBIP340PubKeyFromHex(valPkStr)
		if err != nil {
			return fmt.Errorf("invalid validator public key %s: %w", valPkStr, err)
		}
		if err := valApp.StartHandlingValidator(valPk, passphrase); err != nil {
			return fmt.Errorf("failed to start the validator instance %s: %w", valPkStr, err)
		}
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	valServer := service.NewValidatorServer(cfg, cfgLogger, valApp, shutdownInterceptor)

	return valServer.RunUntilShutdown()
}
