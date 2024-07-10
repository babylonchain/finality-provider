package main

import (
	"fmt"
	"os"

	"github.com/babylonchain/babylon/app"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/babylonchain/finality-provider/finality-provider/cmd/fpd/daemon"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
)

// NewRootCmd creates a new root command for fpd. It is called once in the main function.
func NewRootCmd() *cobra.Command {
	var (
		clientCtx client.Context
	)

	rootCmd := &cobra.Command{
		Use:           "fpd",
		Short:         "fpd - Finality Provider Daemon (fpd).",
		Long:          `fpd is the daemon to create and manage finality providers.`,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// TODO(verify): if it uses the default encoding config it fails to list keys! output:
			// "xx" is not a valid name or address: unable to unmarshal item.Data:
			// Bytes left over in UnmarshalBinaryLengthPrefixed, should read 10 more bytes but have 154
			// [cosmos/cosmos-sdk@v0.50.6/crypto/keyring/keyring.go:973
			tempApp := app.NewTmpBabylonApp()

			clientCtx = clientCtx.
				WithCodec(tempApp.AppCodec()).
				WithInterfaceRegistry(tempApp.InterfaceRegistry()).
				WithTxConfig(tempApp.TxConfig()).
				WithLegacyAmino(tempApp.LegacyAmino()).
				WithInput(os.Stdin)

			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			clientCtx = clientCtx.WithCmdContext(cmd.Context())
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
	cmd.AddCommand(daemon.CommandKeys(), daemon.CommandStart(), daemon.CommandInit())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your fpd CLI '%s'", err)
		os.Exit(1)
	}
}
