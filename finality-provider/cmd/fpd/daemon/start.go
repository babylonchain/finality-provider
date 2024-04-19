package daemon

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/log"
	"github.com/babylonchain/finality-provider/util"
)

const (
	noBabylonChainFlag = "no-bbn"
)

var StartCommand = cli.Command{
	Name:        "start",
	Usage:       "Start the finality-provider app",
	Description: "Start the finality-provider app. Note that eotsd should be started beforehand",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The pass phrase used to decrypt the private key",
			Value: defaultPassphrase,
		},
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the finality-provider home directory",
			Value: fpcfg.DefaultFpdDir,
		},
		cli.StringFlag{
			Name:  fpPkFlag,
			Usage: "The public key of the finality-provider to start",
		},
		cli.StringFlag{
			Name:  rpcListenerFlag,
			Usage: "The address that the RPC server listens to",
		},
		cli.BoolFlag{
			Name:  noBabylonChainFlag,
			Usage: "If specified, does not connect to client controller",
		},
	},
	Action: start,
}

func start(ctx *cli.Context) error {
	homePath, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = util.CleanAndExpandPath(homePath)

	passphrase := ctx.String(passphraseFlag)
	fpPkStr := ctx.String(fpPkFlag)
	rpcListener := ctx.String(rpcListenerFlag)

	cfg, err := fpcfg.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if rpcListener != "" {
		_, err := net.ResolveTCPAddr("tcp", rpcListener)
		if err != nil {
			return fmt.Errorf("invalid RPC listener address %s, %w", rpcListener, err)
		}
		cfg.RpcListener = rpcListener
	}

	logger, err := log.NewRootLoggerWithFile(fpcfg.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize the logger: %w", err)
	}

	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}

	fpApp, err := loadApp(ctx, logger, cfg, dbBackend)
	if err != nil {
		return fmt.Errorf("failed to load app: %w", err)
	}

	// only start the app without starting any finality-provider instance
	// as there might be no finality-provider registered yet
	if err := fpApp.Start(); err != nil {
		return fmt.Errorf("failed to start the finality-provider app: %w", err)
	}

	if fpPkStr != "" {
		// start the finality-provider instance with the given public key
		fpPk, err := types.NewBIP340PubKeyFromHex(fpPkStr)
		if err != nil {
			return fmt.Errorf("invalid finality-provider public key %s: %w", fpPkStr, err)
		}
		if err := fpApp.StartHandlingFinalityProvider(fpPk, passphrase); err != nil {
			return fmt.Errorf("failed to start the finality-provider instance %s: %w", fpPkStr, err)
		}
	} else {
		if err := fpApp.StartHandlingAll(); err != nil {
			return err
		}
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		return err
	}

	fpServer := service.NewFinalityProviderServer(cfg, logger, fpApp, dbBackend, shutdownInterceptor)

	return fpServer.RunUntilShutdown()
}

// loadApp initialize an finality provider app based on config and flags set.
func loadApp(
	ctx *cli.Context,
	logger *zap.Logger,
	cfg *fpcfg.Config,
	dbBackend walletdb.DB,
) (*service.FinalityProviderApp, error) {
	if ctx.Bool(noBabylonChainFlag) {
		fpApp, err := service.NewFinalityProviderAppWithEotsFromConfig(cfg, dbBackend, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create finality-provider with not bbn chain app: %v", err)
		}

		return fpApp, nil
	}

	fpApp, err := service.NewFinalityProviderAppFromConfig(cfg, dbBackend, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality-provider app: %v", err)
	}

	// sync finality-provider status
	if err := fpApp.SyncFinalityProviderStatus(); err != nil {
		return nil, fmt.Errorf("failed to sync finality-provider status: %w", err)
	}

	return fpApp, nil
}
