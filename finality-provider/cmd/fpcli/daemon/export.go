package daemon

import (
	"encoding/hex"
	"encoding/json"
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

const (
	signedFinalityProviderFlag = "signed-fp"
)

// FinalityProviderSigned wrap the finality provider by adding the
// signed finality probider by the bbn pub key as hex
type FinalityProviderSigned struct {
	btcstktypes.FinalityProvider
	// FpSigHex is the finality provider cosmos sdk chain key
	// can be verified with the pub key in btcstktypes.FinalityProvider.BabylonPk
	FpSigHex string `json:"fp_sig_hex"`
}

var ExportFinalityProvider = cli.Command{
	Name:      "export-finality-provider",
	ShortName: "exfp",
	Usage:     "Creates, stores, and exports one finality provider.",
	Description: `Connects with the EOTS manager defined in config, creates a new
key pair formatted by BIP-340 (Schnorr Signatures), generates the master public
randomness pair, stores the finality provider and exports it by printing the json
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

var VerifyExportedFinalityProvider = cli.Command{
	Name:      "verify-exported-finality-provider",
	ShortName: "vexfp",
	Usage:     "Checks if finality provider is correctly signed.",
	UsageText: `fpcli vexfp --signed-fp '{
	"description": {
		"moniker": "my-fp-nickname",
		"identity": "anyIdentity",
		"website": "www.my-public-available-website.com",
		"security_contact": "your.email@gmail.com",
		"details": "other overall info"
	},
	"commission": "0.050000000000000000",
	"babylon_pk": {
		"key": "AzHiB4T9Za1H7pn9NB95UhUJLQ0vOpUAx82jKUtdkyka"
	},
	"btc_pk": "5affe98cd7b180e2822d4d25fd8fab2dafd6b31a9441b3f6c593022fc4d30e5a",
	"pop": {
		"babylon_sig": "waCl0LFEs8m3vSE6cZoDNb4qRMheZtrGgpvNiZptmb4xueLfvoP8y/b2MqOlBiBSsmfypYni468eICGsO0ITmA==",
		"btc_sig": "KxPlo28i7H9IH3fJAAe/ZsuOYdUkGcEw+nnv1BxgakFycW85xag69js6Q5zmvuO++MFh0JbbZq+lTjneE9tosQ=="
	},
	"master_pub_rand": "xpub661MyMwAqRbcG23M9EWAJw71GYxJWfnU47bqCw9gjnALYB1vPQkG6cnkkxyU1LriBi5JXCZb8XK2r454NSnPRrdVxaZNJs9bVKdj4ff3NkC",
	"fp_sig_hex": "dad8205a2686a38e01bc1d2dd20366981fcd381d0fe2d330ddc415dcb8f507e6407672aac39d121f2d3683ed8ad7bd53241047a1c28d7b163db7c4c5256bc1ba"
}'`,
	Description: `Parses the signed finality provider from input
	as json and verifies if it matches the signature.
	Returns an error message if failed to verify`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  signedFinalityProviderFlag,
			Usage: "The signed finality provider to be verified",
		},
	},
	Action: verifyExportedFp,
}

func verifyExportedFp(ctx *cli.Context) error {
	rawSignedFp := ctx.String(signedFinalityProviderFlag)
	if len(rawSignedFp) == 0 {
		return fmt.Errorf("flag %s is mandatory, set a signed fp", signedFinalityProviderFlag)
	}

	var signedFp FinalityProviderSigned
	if err := json.Unmarshal([]byte(rawSignedFp), &signedFp); err != nil {
		return fmt.Errorf("invalid input %s to parse into FinalityProviderSigned: %w", rawSignedFp, err)
	}

	rawSig, err := hex.DecodeString(signedFp.FpSigHex)
	if err != nil {
		return fmt.Errorf("unable to decode signed fp %s: %w", &signedFp.FpSigHex, err)
	}

	fpbz, err := signedFp.FinalityProvider.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal signed finality provider %+v: %w", signedFp.FinalityProvider, err)
	}

	if !signedFp.BabylonPk.VerifySignature(fpbz, rawSig) {
		return fmt.Errorf("bad signature %s to finality provider: %+v", signedFp.FpSigHex, signedFp.FinalityProvider)
	}
	return nil
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

	stored, chainSk, err := app.StoreFinalityProvider(ctx.String(passphraseFlag), keyName, ctx.String(hdPathFlag), ctx.String(chainIdFlag), &description, &commissionRate)
	if err != nil {
		return err
	}

	fp := btcstktypes.FinalityProvider{
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
	}

	fpbz, err := fp.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal finality provider %+v: %w", fp, err)
	}

	signature, err := chainSk.Sign(fpbz)
	if err != nil {
		return fmt.Errorf("failed to sign finality provider %+v: %w", fp, err)
	}

	printRespJSON(FinalityProviderSigned{
		FinalityProvider: fp,
		FpSigHex:         hex.EncodeToString(signature),
	})

	return nil
}
