package main

import (
	"fmt"
	"os"

	"github.com/babylonchain/finality-provider/eots-verifier/cmd/daemon"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "eots-verifier"
	app.Usage = "EOTS Verifier Daemon (eots-verifier)"
	app.Commands = append(app.Commands, daemon.StartCommand, daemon.InitCommand)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "[eots-verifier] %v\n", err)
		os.Exit(1)
	}
}
