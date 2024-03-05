package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/math"
	bbntypes "github.com/babylonchain/babylon/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/urfave/cli"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	dc "github.com/babylonchain/finality-provider/finality-provider/service/client"
)

var (
	defaultAppHashStr = "fd903d9baeb3ab1c734ee003de75f676c5a9a8d0574647e5385834d57d3e79ec"
)

var getDaemonInfoCmd = cli.Command{
	Name:      "get-info",
	ShortName: "gi",
	Usage:     "Get information of the running daemon.",
	Action:    getInfo,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
	},
}

func getInfo(ctx *cli.Context) error {
	daemonAddress := ctx.String(fpdDaemonAddressFlag)
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

var createFpDaemonCmd = cli.Command{
	Name:      "create-finality-provider",
	ShortName: "cfp",
	Usage:     "Create a finality provider object and save it in database.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
		cli.StringFlag{
			Name:  keyNameFlag,
			Usage: "The unique name of the finality provider key",
		},
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "The home path of the finality provider daemon (fpd)",
			Value: fpcfg.DefaultFpdDir,
		},
		cli.StringFlag{
			Name:     chainIdFlag,
			Usage:    "The identifier of the consumer chain",
			Required: true,
		},
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The pass phrase used to encrypt the keys",
			Value: defaultPassphrase,
		},
		cli.StringFlag{
			Name:  hdPathFlag,
			Usage: "The hd path used to derive the private key",
			Value: defaultHdPath,
		},
		cli.StringFlag{
			Name:  commissionRateFlag,
			Usage: "The commission rate for the finality provider, e.g., 0.05",
			Value: "0.05",
		},
		cli.StringFlag{
			Name:  monikerFlag,
			Usage: "A human-readable name for the finality provider",
			Value: "",
		},
		cli.StringFlag{
			Name:  identityFlag,
			Usage: "An optional identity signature (ex. UPort or Keybase)",
			Value: "",
		},
		cli.StringFlag{
			Name:  websiteFlag,
			Usage: "An optional website link",
			Value: "",
		},
		cli.StringFlag{
			Name:  securityContactFlag,
			Usage: "An optional email for security contact",
			Value: "",
		},
		cli.StringFlag{
			Name:  detailsFlag,
			Usage: "Other optional details",
			Value: "",
		},
	},
	Action: createFpDaemon,
}

func createFpDaemon(ctx *cli.Context) error {
	daemonAddress := ctx.String(fpdDaemonAddressFlag)

	commissionRate, err := math.LegacyNewDecFromStr(ctx.String(commissionRateFlag))
	if err != nil {
		return fmt.Errorf("invalid commission rate: %w", err)
	}

	description, err := getDescriptionFromContext(ctx)
	if err != nil {
		return fmt.Errorf("invalid description: %w", err)
	}

	// we add the following check to ensure that the chain key is created
	// beforehand
	cfg, err := fpcfg.LoadConfig(ctx.String(homeFlag))
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", fpcfg.ConfigFile(ctx.String(homeFlag)), err)
	}

	keyName := ctx.String(keyNameFlag)
	// if key name is not specified, we use the key of the config
	if keyName == "" {
		keyName = cfg.BabylonConfig.Key
		if keyName == "" {
			return fmt.Errorf("the key in config is empty")
		}
	}

	client, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	info, err := client.CreateFinalityProvider(
		context.Background(),
		keyName,
		ctx.String(chainIdFlag),
		ctx.String(passphraseFlag),
		ctx.String(hdPathFlag),
		description,
		&commissionRate,
	)

	if err != nil {
		return err
	}

	printRespJSON(info.FinalityProvider)

	return nil
}

func getDescriptionFromContext(ctx *cli.Context) (stakingtypes.Description, error) {
	// get information for description
	monikerStr := ctx.String(monikerFlag)
	identityStr := ctx.String(identityFlag)
	websiteStr := ctx.String(websiteFlag)
	securityContactStr := ctx.String(securityContactFlag)
	detailsStr := ctx.String(detailsFlag)

	description := stakingtypes.NewDescription(monikerStr, identityStr, websiteStr, securityContactStr, detailsStr)

	return description.EnsureLength()
}

var lsFpDaemonCmd = cli.Command{
	Name:      "list-finality-providers",
	ShortName: "ls",
	Usage:     "List finality providers stored in the database.",
	Action:    lsFpDaemon,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
	},
}

func lsFpDaemon(ctx *cli.Context) error {
	daemonAddress := ctx.String(fpdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	resp, err := rpcClient.QueryFinalityProviderList(context.Background())
	if err != nil {
		return err
	}

	printRespJSON(resp)

	return nil
}

var fpInfoDaemonCmd = cli.Command{
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

var registerFpDaemonCmd = cli.Command{
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
		cli.StringFlag{
			Name:  "force",
			Usage: "Attempt to forcefully register the finality provider",
			Value: "false",
		},
	},
	Action: registerFp,
}

func registerFp(ctx *cli.Context) error {
	if ctx.Bool("force") {
		return forceRegisterFp(ctx)
	}

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

// forceRegisterFp attempts to forcefully register the finality provider.
// It will update the fp status CREATE to REGISTERED in the fpd database,
// if the fp appears to be registered in the Babylon chain.
func forceRegisterFp(ctx *cli.Context) error {
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

	res, err := rpcClient.ForceRegisterFinalityProvider(context.Background(), fpPk)
	if err != nil {
		return err
	}

	printRespJSON(res)

	return nil
}

// addFinalitySigDaemonCmd allows manual submission of finality signatures
// NOTE: should only be used for presentation/testing purposes
var addFinalitySigDaemonCmd = cli.Command{
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
