package cmd

import (
	"os"

	"github.com/babylonchain/babylon/app"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"
)

// PersistClientCtx persist some vars from the cmd to the client context.
func PersistClientCtx(clientCtx client.Context) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
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
