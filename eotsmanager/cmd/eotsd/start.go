package main

import (
	"fmt"
	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	eotsservice "github.com/babylonchain/btc-validator/eotsmanager/service"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:        "start",
	Usage:       "eotsd start",
	Description: "Start the Extractable One Time Signature Daemon.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the eots home directory",
			Value: config.DefaultEOTSDir,
		},
	},
	Action: startFn,
}

func startFn(ctx *cli.Context) error {
	homePath := ctx.String(homeFlag)

	cfg, cfgLogger, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", homePath, err)
	}

	eotsManager, err := eotsmanager.NewLocalEOTSManager(cfg, cfgLogger)
	if err != nil {
		return fmt.Errorf("failed to create EOTS manager: %w", err)
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	eotsServer := eotsservice.NewEOTSManagerServer(cfg, cfgLogger, eotsManager, shutdownInterceptor)

	return eotsServer.RunUntilShutdown()
}
