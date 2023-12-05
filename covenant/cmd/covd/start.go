package main

import (
	"fmt"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	covsrv "github.com/babylonchain/btc-validator/covenant/service"
)

const (
	passphraseFlag = "passphrase"
	homeFlag       = "home"

	defaultPassphrase = ""
)

var startCovenant = cli.Command{
	Name:        "start",
	Usage:       "covd start",
	Description: "Start the Covenant Emulator Daemon. Note that the Covenant should be created beforehand",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The pass phrase used to encrypt the keys",
			Value: defaultPassphrase,
		},
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the covenant home directory",
			Value: covcfg.DefaultCovenantDir,
		},
	},
	Action: startCovenantFn,
}

func startCovenantFn(ctx *cli.Context) error {
	homePath := ctx.String(homeFlag)
	cfg, cfgLogger, err := covcfg.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", homePath, err)
	}

	bbnClient, err := clientcontroller.NewBabylonController(cfg.BabylonConfig, &cfg.ActiveNetParams, cfgLogger)
	if err != nil {
		return fmt.Errorf("failed to create rpc client for the consumer chain: %w", err)
	}

	ce, err := covenant.NewCovenantEmulator(cfg, bbnClient, ctx.String(passphraseFlag), cfgLogger)
	if err != nil {
		return fmt.Errorf("failed to start the covenant emulator: %w", err)
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	srv := covsrv.NewCovenantServer(cfgLogger, ce, shutdownInterceptor)
	if err != nil {
		return fmt.Errorf("failed to create covenant server: %w", err)
	}

	return srv.RunUntilShutdown()
}
