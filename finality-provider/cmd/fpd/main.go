package main

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/spf13/cobra"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[btc-finality-provider] %v\n", err)
	os.Exit(1)
}

var rootCmd = &cobra.Command{
	Use:   "fpd",
	Short: "fpd - Finality Provider Daemon (fpd).",
	Long:  `fpd is the daemon to handle finality provider submission of randomness from BTC to proof of stake chains`,
	// Run: func(cmd *cobra.Command, args []string) {

	// },
}

func main() {
	// app.Commands = append(app.Commands, dcli.StartCommand, dcli.InitCommand)
	// app.Commands = append(app.Commands, dcli.KeysCommands...)
	rootCmd.AddCommand(keys.Commands())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
