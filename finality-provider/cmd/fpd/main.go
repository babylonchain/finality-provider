package main

import (
	"fmt"
	"os"

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
	app.Commands = append(app.Commands, startCommand, initCommand)
	app.Commands = append(app.Commands, keysCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
