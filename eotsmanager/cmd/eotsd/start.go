package main

import (
	"fmt"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
	eotsservice "github.com/babylonchain/finality-provider/eotsmanager/service"
	"github.com/babylonchain/finality-provider/log"
	"github.com/babylonchain/finality-provider/util"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"
	"path/filepath"
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
	homePath, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = util.CleanAndExpandPath(homePath)

	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", homePath, err)
	}

	logger, err := log.NewRootLoggerWithFile(config.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to load the logger")
	}

	eotsManager, err := eotsmanager.NewLocalEOTSManager(homePath, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create EOTS manager: %w", err)
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	eotsServer := eotsservice.NewEOTSManagerServer(cfg, logger, eotsManager, shutdownInterceptor)

	return eotsServer.RunUntilShutdown()
}
