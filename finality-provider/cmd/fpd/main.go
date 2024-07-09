package main

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/spf13/cobra"

	appparams "github.com/babylonchain/babylon/app/params"
	"github.com/babylonchain/finality-provider/finality-provider/cmd/fpd/daemon"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
)

// NewRootCmd creates a new root command for fpd. It is called once in the main function.
func NewRootCmd() *cobra.Command {
	var (
		clientCtx client.Context
	)
	encCfg := appparams.DefaultEncodingConfig()

	rootCmd := &cobra.Command{
		Use:           "fpd",
		Short:         "fpd - Finality Provider Daemon (fpd).",
		Long:          `fpd is the daemon to create and manage finality providers.`,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			clientCtx = clientCtx.WithCmdContext(cmd.Context())
			clientCtx.Codec = encCfg.Codec
			if err := client.SetCmdClientContextHandler(clientCtx, cmd); err != nil {
				return err
			}

			return nil
		},
	}
	rootCmd.PersistentFlags().String(flags.FlagHome, fpcfg.DefaultFpdDir, "The application home directory")

	return rootCmd
}

func main() {
	cmd := NewRootCmd()
	cmd.AddCommand(keys.Commands(), daemon.CommandStart(), daemon.CommandInit())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your fpd CLI '%s'", err)
		os.Exit(1)
	}
}
