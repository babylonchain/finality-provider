package main

import (
	"fmt"
	"os"

	dcli "github.com/babylonchain/finality-provider/finality-provider/cmd/fpd/daemon"
	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[btc-finality-provider] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "fpd"
	app.Usage = "Finality Provider Daemon (fpd)."
	app.Commands = append(app.Commands, dcli.StartCommand, dcli.InitCommand)
	app.Commands = append(app.Commands, dcli.KeysCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
