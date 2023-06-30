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

func main() {
	app := cli.NewApp()
	app.Name = "valcli"
	app.Usage = "control plane for your BTC Validator Daemon (vald)"
	app.Flags = []cli.Flag{
		// 	TODO add flags
	}

	app.Commands = append(app.Commands, validatorsCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
