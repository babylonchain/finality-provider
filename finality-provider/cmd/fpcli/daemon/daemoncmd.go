package daemon

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/urfave/cli"

	fpcmd "github.com/babylonchain/finality-provider/finality-provider/cmd"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	dc "github.com/babylonchain/finality-provider/finality-provider/service/client"
)

var (
	defaultFpdDaemonAddress = "127.0.0.1:" + strconv.Itoa(fpcfg.DefaultRPCPort)
	defaultAppHashStr       = "fd903d9baeb3ab1c734ee003de75f676c5a9a8d0574647e5385834d57d3e79ec"
)

// CommandGetDaemonInfo returns the get-info command by connecting to the fpd daemon.
func CommandGetDaemonInfo() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "get-info",
		Aliases: []string{"gi"},
		Short:   "Get information of the running fpd daemon.",
		Example: fmt.Sprintf(`fpcli get-info --daemon-address %s`, defaultFpdDaemonAddress),
		Args:    cobra.NoArgs,
		RunE:    runCommandGetDaemonInfo,
	}
	cmd.Flags().String(fpdDaemonAddressFlag, defaultFpdDaemonAddress, "The RPC server address of fpd")
	return cmd
}

func runCommandGetDaemonInfo(cmd *cobra.Command, args []string) error {
	daemonAddress, err := cmd.Flags().GetString(fpdDaemonAddressFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", fpdDaemonAddressFlag, err)
	}

	client, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	info, err := client.GetInfo(context.Background())
	if err != nil {
		return err
	}

	printRespJSON(info)
	return nil
}

// CommandCreateFP returns the create-finality-provider command by connecting to the fpd daemon.
func CommandCreateFP() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "create-finality-provider",
		Aliases: []string{"cfp"},
		Short:   "Create a finality provider object and save it in database.",
		Example: fmt.Sprintf(`fpcli create-finality-provider --daemon-address %s`, defaultFpdDaemonAddress),
		Args:    cobra.NoArgs,
		RunE:    fpcmd.RunEWithClientCtx(runCommandCreateFP),
	}

	f := cmd.Flags()
	f.String(fpdDaemonAddressFlag, defaultFpdDaemonAddress, "The RPC server address of fpd")
	f.String(keyNameFlag, "", "The unique name of the finality provider key")
	f.String(sdkflags.FlagHome, fpcfg.DefaultFpdDir, "The application home directory")
	f.String(chainIdFlag, "", "The identifier of the consumer chain")
	f.String(passphraseFlag, "", "The pass phrase used to encrypt the keys")
	f.String(hdPathFlag, "", "The hd path used to derive the private key")
	f.String(commissionRateFlag, "0.05", "The commission rate for the finality provider, e.g., 0.05")
	f.String(monikerFlag, "", "A human-readable name for the finality provider")
	f.String(identityFlag, "", "An optional identity signature (ex. UPort or Keybase)")
	f.String(websiteFlag, "", "An optional website link")
	f.String(securityContactFlag, "", "An email for security contact")
	f.String(detailsFlag, "", "Other optional details")

	return cmd
}

func runCommandCreateFP(ctx client.Context, cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	daemonAddress, err := flags.GetString(fpdDaemonAddressFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", fpdDaemonAddressFlag, err)
	}

	commissionRateStr, err := flags.GetString(commissionRateFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", commissionRateFlag, err)
	}
	commissionRate, err := math.LegacyNewDecFromStr(commissionRateStr)
	if err != nil {
		return fmt.Errorf("invalid commission rate: %w", err)
	}

	description, err := getDescriptionFromFlags(flags)
	if err != nil {
		return fmt.Errorf("invalid description: %w", err)
	}

	keyName, err := loadKeyName(ctx, cmd)
	if err != nil {
		return fmt.Errorf("not able to load key name: %w", err)
	}

	client, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	chainId, err := flags.GetString(chainIdFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", chainIdFlag, err)
	}

	passphrase, err := flags.GetString(passphraseFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", passphraseFlag, err)
	}

	hdPath, err := flags.GetString(hdPathFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", hdPathFlag, err)
	}

	info, err := client.CreateFinalityProvider(
		context.Background(),
		keyName,
		chainId,
		passphrase,
		hdPath,
		description,
		&commissionRate,
	)
	if err != nil {
		return err
	}

	printRespJSON(info.FinalityProvider)
	return nil
}

func getDescriptionFromFlags(f *pflag.FlagSet) (desc stakingtypes.Description, err error) {
	// get information for description
	monikerStr, err := f.GetString(monikerFlag)
	if err != nil {
		return desc, fmt.Errorf("failed to read flag %s: %w", monikerFlag, err)
	}
	identityStr, err := f.GetString(identityFlag)
	if err != nil {
		return desc, fmt.Errorf("failed to read flag %s: %w", identityFlag, err)
	}
	websiteStr, err := f.GetString(websiteFlag)
	if err != nil {
		return desc, fmt.Errorf("failed to read flag %s: %w", websiteFlag, err)
	}
	securityContactStr, err := f.GetString(securityContactFlag)
	if err != nil {
		return desc, fmt.Errorf("failed to read flag %s: %w", securityContactFlag, err)
	}
	detailsStr, err := f.GetString(detailsFlag)
	if err != nil {
		return desc, fmt.Errorf("failed to read flag %s: %w", detailsFlag, err)
	}

	description := stakingtypes.NewDescription(monikerStr, identityStr, websiteStr, securityContactStr, detailsStr)
	return description.EnsureLength()
}

// CommandLsFP returns the list-finality-providers command by connecting to the fpd daemon.
func CommandLsFP() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "list-finality-providers",
		Aliases: []string{"ls"},
		Short:   "List finality providers stored in the database.",
		Example: fmt.Sprintf(`fpcli list-finality-providers --daemon-address %s`, defaultFpdDaemonAddress),
		Args:    cobra.NoArgs,
		RunE:    runCommandLsFP,
	}
	cmd.Flags().String(fpdDaemonAddressFlag, defaultFpdDaemonAddress, "The RPC server address of fpd")
	return cmd
}

func runCommandLsFP(cmd *cobra.Command, args []string) error {
	daemonAddress, err := cmd.Flags().GetString(fpdDaemonAddressFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", fpdDaemonAddressFlag, err)
	}

	client, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	resp, err := client.QueryFinalityProviderList(context.Background())
	if err != nil {
		return err
	}
	printRespJSON(resp)

	return nil
}

var FpInfoDaemonCmd = cli.Command{
	Name:      "finality-provider-info",
	ShortName: "fpi",
	Usage:     "Show the information of the finality provider.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
		cli.StringFlag{
			Name:     fpBTCPkFlag,
			Usage:    "The hex string of the BTC public key",
			Required: true,
		},
	},
	Action: fpInfoDaemon,
}

func fpInfoDaemon(ctx *cli.Context) error {
	daemonAddress := ctx.String(fpdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(ctx.String(fpBTCPkFlag))
	if err != nil {
		return err
	}

	resp, err := rpcClient.QueryFinalityProviderInfo(context.Background(), fpPk)
	if err != nil {
		return err
	}

	printRespJSON(resp.FinalityProvider)

	return nil
}

var RegisterFpDaemonCmd = cli.Command{
	Name:      "register-finality-provider",
	ShortName: "rfp",
	Usage:     "Register a created finality provider to Babylon.",
	UsageText: fmt.Sprintf("register-finality-provider --%s [btc-pk]", fpBTCPkFlag),
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
		cli.StringFlag{
			Name:     fpBTCPkFlag,
			Usage:    "The hex string of the finality provider BTC public key",
			Required: true,
		},
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The pass phrase used to encrypt the keys",
			Value: defaultPassphrase,
		},
	},
	Action: registerFp,
}

func registerFp(ctx *cli.Context) error {
	fpPkStr := ctx.String(fpBTCPkFlag)
	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(fpPkStr)
	if err != nil {
		return fmt.Errorf("invalid BTC public key: %w", err)
	}

	daemonAddress := ctx.String(fpdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	res, err := rpcClient.RegisterFinalityProvider(context.Background(), fpPk, ctx.String(passphraseFlag))
	if err != nil {
		return err
	}

	printRespJSON(res)

	return nil
}

// AddFinalitySigDaemonCmd allows manual submission of finality signatures
// NOTE: should only be used for presentation/testing purposes
var AddFinalitySigDaemonCmd = cli.Command{
	Name:      "add-finality-sig",
	ShortName: "afs",
	Usage:     "Send a finality signature to the consumer chain. This command should only be used for presentation/testing purposes",
	UsageText: fmt.Sprintf("add-finality-sig --%s [btc_pk_hex]", fpBTCPkFlag),
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
		cli.StringFlag{
			Name:     fpBTCPkFlag,
			Usage:    "The hex string of the BTC public key",
			Required: true,
		},
		cli.Uint64Flag{
			Name:     blockHeightFlag,
			Usage:    "The height of the chain block",
			Required: true,
		},
		cli.StringFlag{
			Name:  appHashFlag,
			Usage: "The last commit hash of the chain block",
			Value: defaultAppHashStr,
		},
	},
	Action: addFinalitySig,
}

func addFinalitySig(ctx *cli.Context) error {
	daemonAddress := ctx.String(fpdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	fpPk, err := bbntypes.NewBIP340PubKeyFromHex(ctx.String(fpBTCPkFlag))
	if err != nil {
		return err
	}

	appHash, err := hex.DecodeString(ctx.String(appHashFlag))
	if err != nil {
		return err
	}

	res, err := rpcClient.AddFinalitySignature(
		context.Background(), fpPk.MarshalHex(), ctx.Uint64(blockHeightFlag), appHash)
	if err != nil {
		return err
	}

	printRespJSON(res)

	return nil
}

func printRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}

func loadKeyName(ctx client.Context, cmd *cobra.Command) (string, error) {
	keyName, err := cmd.Flags().GetString(keyNameFlag)
	if err != nil {
		return "", fmt.Errorf("failed to read flag %s: %w", keyNameFlag, err)
	}
	// if key name is not specified, we use the key of the config
	if keyName != "" {
		return keyName, nil
	}

	// we add the following check to ensure that the chain key is created
	// beforehand
	cfg, err := fpcfg.LoadConfig(ctx.HomeDir)
	if err != nil {
		return "", fmt.Errorf("failed to load config from %s: %w", fpcfg.ConfigFile(ctx.HomeDir), err)
	}

	keyName = cfg.BabylonConfig.Key
	if keyName == "" {
		return "", fmt.Errorf("the key in config is empty")
	}
	return keyName, nil
}
