package daemon

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	dc "github.com/babylonchain/finality-provider/finality-provider/service/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/x/staking/types"

	bbn "github.com/babylonchain/babylon/types"
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
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
		cli.StringFlag{
			Name:     fpBTCPkFlag,
			Usage:    "The hex string of the BTC public key",
			Required: true,
		},
		cli.BoolFlag{
			Name: signedFlag,
			Usage: `Defines if needs to sign the exported finality provider,
			if true, it is necessary to define keyring flags`,
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
			Name:  passphraseFlag,
			Usage: "The pass phrase used to encrypt the keys",
			Value: defaultPassphrase,
		},
		cli.StringFlag{
			Name:  hdPathFlag,
			Usage: "The hd path used to derive the private key",
			Value: defaultHdPath,
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
		return fmt.Errorf("unable to decode signed fp %s: %w", signedFp.FpSigHex, err)
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
	daemonAddress := ctx.String(fpdDaemonAddressFlag)
	client, cleanUp, err := dc.NewFinalityProviderServiceGRpcClient(daemonAddress)
	if err != nil {
		return fmt.Errorf("failled to connect to daemon addr %s: %w", daemonAddress, err)
	}
	defer cleanUp()

	fpBtcPkHex := ctx.String(fpBTCPkFlag)
	fpPk, err := bbn.NewBIP340PubKeyFromHex(fpBtcPkHex)
	if err != nil {
		return fmt.Errorf("invalid fp btc pk hex %s: %w", fpBtcPkHex, err)
	}

	fpInfoResp, err := client.QueryFinalityProviderInfo(context.Background(), fpPk)
	if err != nil {
		return fmt.Errorf("failed to query fp info from %s: %w", fpBtcPkHex, err)
	}

	fpInfo := fpInfoResp.FinalityProvider
	comm, err := math.LegacyNewDecFromStr(fpInfo.Commission)
	if err != nil {
		return fmt.Errorf("failed to parse fp commission %s: %w", fpInfo.Commission, err)
	}

	cosmosRawPubKey, err := hex.DecodeString(fpInfo.ChainPkHex)
	if err != nil {
		return fmt.Errorf("failed to decode chain pk hex %s: %w", fpInfo.ChainPkHex, err)
	}

	cosmosPubKey := &secp256k1.PubKey{
		Key: cosmosRawPubKey,
	}

	desc := fpInfo.Description
	fp := btcstktypes.FinalityProvider{
		BtcPk:         fpPk,
		MasterPubRand: fpInfo.MasterPubRand,
		Pop: &btcstktypes.ProofOfPossession{
			BtcSigType: btcstktypes.BTCSigType_BIP340,
			BabylonSig: fpInfo.Pop.ChainSig,
			BtcSig:     fpInfo.Pop.BtcSig,
		},
		BabylonPk: cosmosPubKey,
		Description: &types.Description{
			Moniker:         desc.Moniker,
			Identity:        desc.Identity,
			Website:         desc.Website,
			SecurityContact: desc.SecurityContact,
			Details:         desc.Details,
		},
		Commission: &comm,
	}

	if !ctx.Bool(signedFlag) {
		printRespJSON(fp)
		return nil
	}

	keyName, err := loadKeyName(ctx)
	if err != nil {
		return fmt.Errorf("not able to load key name: %w", err)
	}

	// sign the finality provider data.
	fpbz, err := fp.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal finality provider %+v: %w", fp, err)
	}

	resp, err := client.SignMessageFromChainKey(
		context.Background(),
		keyName,
		ctx.String(passphraseFlag),
		ctx.String(hdPathFlag),
		fpbz,
	)
	if err != nil {
		return fmt.Errorf("failed to sign finality provider: %w", err)
	}

	printRespJSON(FinalityProviderSigned{
		FinalityProvider: fp,
		FpSigHex:         hex.EncodeToString(resp.Signature),
	})

	return nil
}
