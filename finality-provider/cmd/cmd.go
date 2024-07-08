package cmd

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/spf13/cobra"

	appparams "github.com/babylonchain/babylon/app/params"
)

// PersistClientCtx persist some vars from the cmd to the client context.
func PersistClientCtx(clientCtx client.Context) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		encCfg := appparams.DefaultEncodingConfig()
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
	}
}

// RunEWithClientCtx runs cmd with client context and returns an error.
func RunEWithClientCtx(
	fRunWithCtx func(ctx client.Context, cmd *cobra.Command, args []string) error,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		clientCtx, err := client.GetClientQueryContext(cmd)
		if err != nil {
			return err
		}

		return fRunWithCtx(clientCtx, cmd, args)
	}
}
