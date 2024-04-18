package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/math"
	eotsclient "github.com/babylonchain/finality-provider/eotsmanager/client"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/log"
	"github.com/babylonchain/finality-provider/util"

	btcstktypes "github.com/babylonchain/babylon/x/btcstaking/types"

	"github.com/urfave/cli"
)

var ExportFinalityProvider = cli.Command{
	Name:      "export-finality-provider",
	ShortName: "exfpd",
	Usage:     "Create, stores, and exports one finality provider.",
	Description: `Connects with the EOTS manager defined in config, creates a new
key pair formatted by BIP-340 (Schnorr Signatures), generates the master public
randomness pair, stores the finality provider and export it by printing the json
structure on the stdout`,
	Flags: []cli.Flag{
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
	Action: exportFp,
}

func exportFp(ctx *cli.Context) error {
	commissionRate, err := math.LegacyNewDecFromStr(ctx.String(commissionRateFlag))
	if err != nil {
		return fmt.Errorf("invalid commission rate: %w", err)
	}

	description, err := getDescriptionFromContext(ctx)
	if err != nil {
		return fmt.Errorf("invalid description: %w", err)
	}

	homePath, err := filepath.Abs(ctx.String(homeFlag))
	if err != nil {
		return err
	}
	homePath = util.CleanAndExpandPath(homePath)

	// we add the following check to ensure that the chain key is created
	// beforehand
	cfg, err := fpcfg.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", fpcfg.ConfigFile(ctx.String(homeFlag)), err)
	}

	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}

	// if the EOTSManagerAddress is empty, run a local EOTS manager;
	// otherwise connect a remote one with a gRPC client
	em, err := eotsclient.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	if err != nil {
		return fmt.Errorf("failed to create EOTS manager client: %w", err)
	}

	logFilePath := filepath.Join(fpcfg.LogDir(homePath), "fpd.export.log")
	if err := util.MakeDirectory(filepath.Dir(logFilePath)); err != nil {
		return err
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logger, err := log.NewRootLogger("console", cfg.LogLevel, logFile)
	if err != nil {
		return fmt.Errorf("failed to initialize the logger: %w", err)
	}

	app, err := service.NewFinalityProviderApp(cfg, nil, em, dbBackend, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize the app: %w", err)
	}

	keyName := ctx.String(keyNameFlag)
	// if key name is not specified, we use the key of the config
	if keyName == "" {
		keyName = cfg.BabylonConfig.Key
		if keyName == "" {
			return fmt.Errorf("the key in config is empty")
		}
	}

	stored, err := app.StoreFinalityProvider(ctx.String(passphraseFlag), keyName, ctx.String(hdPathFlag), ctx.String(chainIdFlag), &description, &commissionRate)
	if err != nil {
		return err
	}

	printRespJSON(btcstktypes.FinalityProvider{
		BtcPk:         stored.GetBIP340BTCPK(),
		MasterPubRand: stored.MasterPubRand,
		Pop: &btcstktypes.ProofOfPossession{
			BtcSigType: btcstktypes.BTCSigType_BIP340,
			BabylonSig: stored.Pop.ChainSig,
			BtcSig:     stored.Pop.BtcSig,
		},
		BabylonPk:            stored.ChainPk,
		Description:          stored.Description,
		Commission:           stored.Commission,
		RegisteredEpoch:      0,
		SlashedBabylonHeight: 0,
		SlashedBtcHeight:     0,
	})

	return nil
}
