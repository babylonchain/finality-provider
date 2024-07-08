package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	fpcmd "github.com/babylonchain/finality-provider/finality-provider/cmd"
	"github.com/babylonchain/finality-provider/finality-provider/cmd/fpcli/daemon"
)

// NewRootCmd creates a new root command for fpd. It is called once in the main function.
func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "fpcli",
		Short:             "fpcli - Control plane for the Finality Provider Daemon (fpd).",
		Long:              `fpcli can create requests and make actions that evolve the Finality Provider Daemon (fpd).`,
		SilenceErrors:     false,
		PersistentPreRunE: fpcmd.PersistClientCtx(client.Context{}),
	}
}

func main() {
	cmd := NewRootCmd()
	cmd.AddCommand(daemon.CommandGetDaemonInfo())
	// 	app.Commands = append(app.Commands,
	// 		dcli.GetDaemonInfoCmd,
	// 		dcli.CreateFpDaemonCmd,
	// 		dcli.LsFpDaemonCmd,
	// 		dcli.FpInfoDaemonCmd,
	// 		dcli.RegisterFpDaemonCmd,
	// 		dcli.AddFinalitySigDaemonCmd,
	// 		dcli.ExportFinalityProvider,
	// 	)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your fpcli '%s'", err)
		os.Exit(1)
	}
}
