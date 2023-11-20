package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[covenant-emulator cli] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "covcli"
	app.Usage = "Control plane for the Covenant Emulator Daemon (covd)."
	app.Commands = append(app.Commands, adminCommands...)
	app.Commands = append(app.Commands, createCovenant)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
