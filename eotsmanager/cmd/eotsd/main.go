package main

import (
	"fmt"
	"github.com/urfave/cli"
	"os"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[eotsd] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "eotsd"
	app.Usage = "Extractable One Time Signature Daemon (eotsd)."
	app.Commands = append(app.Commands, startEots)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
