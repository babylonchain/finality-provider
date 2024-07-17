package daemon

import (
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	btcstakingcli "github.com/babylonchain/babylon/x/btcstaking/client/cli"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	authcli "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
)

// CommandTxs returns the transaction commands for finality provider related msgs.
func CommandTxs() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcli.GetSignCommand(),
		btcstakingcli.NewCreateFinalityProviderCmd(),
		NewValidateSignedFinalityProviderCmd(),
	)

	return cmd
}

// NewValidateSignedFinalityProviderCmd returns the command line for
// tx validate-signed-finality-provider
func NewValidateSignedFinalityProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-signed-finality-provider [file_path_signed_msg]",
		Args:  cobra.ExactArgs(1),
		Short: "Validates if the signed finality provider is valid",
		Long: strings.TrimSpace(`
			Loads the signed msg MsgCreateFinalityProvider and checks if the basic
			information is satisfied and the Proof of Possession is in valid by the
			signer of the msg and the finality provider address
		`),
		Example: strings.TrimSpace(
			`fdp tx validate-signed-finality-provider ./path/to/signed-msg.json`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			networkBTC, err := cmd.Flags().GetString(btcNetworkFlag)
			if err != nil {
				return err
			}

			var btcNetCfg chaincfg.Params
			if len(networkBTC) == 0 { // not set in flag, load from config
				cfg, err := fpcfg.LoadConfig(ctx.HomeDir)
				if err != nil {
					return fmt.Errorf("failed to load configuration at %s: %w", ctx.HomeDir, err)
				}
				btcNetCfg = cfg.BTCNetParams
			} else {
				btcNetCfg, err = fpcfg.NetParamsBTC(networkBTC)
				if err != nil {
					return err
				}
			}

			stdTx, err := authclient.ReadTxFromFile(ctx, args[0])
			if err != nil {
				return err
			}

			msgsV2, err := stdTx.GetMsgsV2()
			if err != nil {
				return err
			}

			msgs := stdTx.GetMsgs()
			for i, sdkMsg := range msgs {
				msgV2 := msgsV2[i]
				msg, ok := sdkMsg.(*btcstakingtypes.MsgCreateFinalityProvider)
				if !ok {
					return fmt.Errorf("unable to parse %+v to MsgCreateFinalityProvider", msg)
				}

				if err := msg.ValidateBasic(); err != nil {
					return fmt.Errorf("error validating basic msg: %w", err)
				}

				signers, err := ctx.Codec.GetMsgV2Signers(msgV2)
				if err != nil {
					return fmt.Errorf("error %w failed to get signers from msg: %+v", err, msg)
				}

				if len(signers) == 0 {
					return fmt.Errorf("no signer at msg %+v", msgV2)
				}

				addrStr, err := ctx.Codec.InterfaceRegistry().SigningContext().AddressCodec().BytesToString(signers[0])
				if err != nil {
					return err
				}

				bbnAddr, err := sdk.AccAddressFromBech32(addrStr)
				if err != nil {
					return fmt.Errorf("invalid signer address %s, please sign with a valid bbn address, err: %w", addrStr, err)
				}

				if err := msg.Pop.Verify(bbnAddr, msg.BtcPk, &btcNetCfg); err != nil {
					return fmt.Errorf("invalid verification of Proof of Possession %w, signer %s", err, bbnAddr.String())
				}
			}

			return nil
		},
	}

	cmd.Flags().String(btcNetworkFlag, "", "The BTC network to use, one of ['mainnet', 'testnet', 'regtest', 'simnet', 'signet'] (it loads the one set in config if is found)")

	return cmd
}
