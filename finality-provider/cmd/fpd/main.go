package main

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/spf13/cobra"

	appparams "github.com/babylonchain/babylon/app/params"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
)

// NewRootCmd creates a new root command for simd. It is called once in the main function.
func NewRootCmd() *cobra.Command {
	var (
		clientCtx client.Context
	)
	encCfg := appparams.DefaultEncodingConfig()

	rootCmd := &cobra.Command{
		Use:           "fpd",
		Short:         "fpd - Finality Provider Daemon (fpd).",
		Long:          `fpd is the daemon to handle finality provider submission of randomness from BTC to proof of stake chains`,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			clientCtx = clientCtx.WithCmdContext(cmd.Context()).WithViper("")
			clientCtx.Codec = encCfg.Codec
			clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			clientCtx, err = config.ReadFromClientConfig(clientCtx)
			if err != nil {
				return err
			}

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
	cmd.AddCommand(keys.Commands())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your fpd CLI '%s'", err)
		os.Exit(1)
	}
}
