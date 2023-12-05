package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/signal"
	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	eotsservice "github.com/babylonchain/btc-validator/eotsmanager/service"
)

func main() {
	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg, cfgLogger, err := config.LoadConfig()
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

	eotsManager, err := eotsmanager.NewLocalEOTSManager(cfg, cfgLogger)
	if err != nil {
		cfgLogger.Fatal("failed to create EOTS manager", zap.Error(err))
	}

	eotsServer := eotsservice.NewEOTSManagerServer(cfg, cfgLogger, eotsManager, shutdownInterceptor)

	if err := eotsServer.RunUntilShutdown(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
