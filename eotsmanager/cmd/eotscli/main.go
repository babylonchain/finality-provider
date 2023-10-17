package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[btc-validator] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "eotscli"
	app.Usage = "Control plane for the EOTS Manager Daemon (eotsd)."
	app.Commands = append(app.Commands, adminCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
