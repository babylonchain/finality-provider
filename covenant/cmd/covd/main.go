package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[covenant-emulator] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "covd"
	app.Usage = "Covenant Emulator Daemon (covd)."
	app.Commands = append(app.Commands, startCovenant)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
