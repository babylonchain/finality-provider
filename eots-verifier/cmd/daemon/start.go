package daemon

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/babylonchain/finality-provider/eots-verifier/config"
	"github.com/babylonchain/finality-provider/eots-verifier/service"
	"github.com/babylonchain/finality-provider/log"
	"github.com/babylonchain/finality-provider/util"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/urfave/cli/v2"
)

var StartCommand = &cli.Command{
	Name:  "start",
	Usage: "Start EOTS Verifier Daemon.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  homeFlag,
			Usage: "The path to the eots-verifier home directory",
			Value: config.DefaultDir,
		},
		&cli.StringFlag{
			Name:  rpcListenerFlag,
			Usage: "The address that the RPC server listens to",
		},
	},
	Action: startFn,
}

func startFn(ctx *cli.Context) error {
	path, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return fmt.Errorf("failed to load home flag: %w", err)
	}
	homePath := util.CleanAndExpandPath(path)
	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", homePath, err)
	}

	rpcListener := ctx.String(rpcListenerFlag)
	if rpcListener != "" {
		_, err := net.ResolveTCPAddr("tcp", rpcListener)
		if err != nil {
			return fmt.Errorf("invalid RPC listener address %s, %w", rpcListener, err)
		}
		cfg.RpcListener = rpcListener
	}

	logger, err := log.NewRootLoggerWithFile(config.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to load the logger")
	}

	shutdown, err := signal.Intercept()
	if err != nil {
		return err
	}

	server, err := service.NewServer(ctx.Context, logger, cfg, shutdown)
	if err != nil {
		return err
	}

	return server.Run()
}
