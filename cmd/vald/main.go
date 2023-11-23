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
	app.Name = "vald"
	app.Usage = "BTC Validator Daemon (vald)."
	app.Commands = append(app.Commands, startValidator)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
