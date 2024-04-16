package main

import (
	"fmt"
	"os"

	dcli "github.com/babylonchain/finality-provider/finality-provider/cmd/fpcli/daemon"
	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[fpd] %v\n", err)
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "fpcli"
	app.Usage = "Control plane for the Finality Provider Daemon (fpd)."

	app.Commands = append(app.Commands,
		dcli.GetDaemonInfoCmd,
		dcli.CreateFpDaemonCmd,
		dcli.LsFpDaemonCmd,
		dcli.FpInfoDaemonCmd,
		dcli.RegisterFpDaemonCmd,
		dcli.AddFinalitySigDaemonCmd,
		dcli.Phase1ExportFinalityProvider,
	)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
