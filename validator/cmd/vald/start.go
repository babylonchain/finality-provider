package main

import (
	"fmt"
	"github.com/babylonchain/babylon/types"
	"github.com/babylonchain/btc-validator/log"
	"github.com/babylonchain/btc-validator/util"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"

	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/babylonchain/btc-validator/validator/service"
)

var startCommand = cli.Command{
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
			Name:  homeFlag,
			Usage: "The path to the validator home directory",
			Value: valcfg.DefaultValdDir,
		},
		cli.StringFlag{
			Name:  valPkFlag,
			Usage: "The public key of the validator to start",
		},
	},
	Action: start,
}

func start(ctx *cli.Context) error {
	homePath := util.CleanAndExpandPath(ctx.String(homeFlag))
	passphrase := ctx.String(passphraseFlag)
	valPkStr := ctx.String(valPkFlag)

	cfg, err := valcfg.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger, err := log.NewRootLoggerWithFile(valcfg.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize the logger")
	}

	valApp, err := service.NewValidatorAppFromConfig(homePath, cfg, logger)
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

	valServer := service.NewValidatorServer(cfg, logger, valApp, shutdownInterceptor)

	return valServer.RunUntilShutdown()
}
