package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/signal"

	babylonclient "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/valcfg"
)

func main() {
	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg, cfgLogger, err := valcfg.LoadConfig()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			// Print error if not due to help request.
			err = fmt.Errorf("failed to load config: %w", err)
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Help was requested, exit normally.
		os.Exit(0)
	}

	bbnClient, err := babylonclient.NewBabylonController(cfg.BabylonConfig, cfgLogger)
	if err != nil {
		cfgLogger.Errorf("failed to create Babylon rpc client: %v", err)
		os.Exit(1)
	}

	valApp, err := service.NewValidatorAppFromConfig(cfg, cfgLogger, bbnClient)
	if err != nil {
		cfgLogger.Errorf("failed to create validator app: %v", err)
		os.Exit(1)
	}
	// start app event loop
	// TODO: graceful shutdown
	if err := valApp.Start(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	valServer := service.NewValidatorServer(cfg, cfgLogger, valApp, shutdownInterceptor)

	err = valServer.RunUntilShutdown()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
