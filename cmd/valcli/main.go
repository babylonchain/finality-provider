package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[btc-staker] %v\n", err)
	os.Exit(1)
}

const (
	btcNetworkFlag = "btc-network"
)

func main() {
	app := cli.NewApp()
	app.Name = "valcli"
	app.Usage = "control plane for your BTC Validator Daemon (vald)"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  btcNetworkFlag,
			Usage: "btc network on which staking should take place",
			Value: "testnet3",
		},
	}

	app.Commands = append(app.Commands, validatorsCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
