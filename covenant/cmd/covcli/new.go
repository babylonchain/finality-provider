package main

import (
	"encoding/json"
	"fmt"

	"github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"github.com/urfave/cli"

	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/val"
)

const (
	keyNameFlag           = "key-name"
	passphraseFlag        = "passphrase"
	hdPathFlag            = "hd-path"
	chainIdFlag           = "chain-id"
	keyringDirFlag        = "keyring-dir"
	keyringBackendFlag    = "keyring-backend"
	defaultChainID        = "chain-test"
	defaultKeyringBackend = keyring.BackendTest
	defaultPassphrase     = ""
	defaultHdPath         = ""
)

type covenantKey struct {
	Name      string `json:"name"`
	PublicKey string `json:"public-key"`
}

var createCovenant = cli.Command{
	Name:      "create-covenant",
	ShortName: "cc",
	Usage:     "Create a Covenant account in the keyring.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  chainIdFlag,
			Usage: "The chainID of the consumer chain",
			Value: defaultChainID,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Usage:    "The unique name of the Covenant key",
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
			Name:  keyringBackendFlag,
			Usage: "Select keyring's backend",
			Value: defaultKeyringBackend,
		},
		cli.StringFlag{
			Name:  keyringDirFlag,
			Usage: "The directory where the keyring is stored",
			Value: covcfg.DefaultCovenantDir,
		},
	},
	Action: createCovenantKey,
}

func createCovenantKey(ctx *cli.Context) error {
	sdkCtx, err := service.CreateClientCtx(
		ctx.String(keyringDirFlag),
		ctx.String(chainIdFlag),
	)
	if err != nil {
		return err
	}

	krController, err := val.NewChainKeyringController(
		sdkCtx,
		ctx.String(keyNameFlag),
		ctx.String(keyringBackendFlag),
	)
	if err != nil {
		return err
	}

	sdkCovenantPk, err := krController.CreateChainKey(ctx.String(passphraseFlag), ctx.String(hdPathFlag))
	if err != nil {
		return fmt.Errorf("failed to create Covenant key: %w", err)
	}

	covenantPk, err := secp256k1.ParsePubKey(sdkCovenantPk.Key)
	if err != nil {
		return err
	}

	bip340Key := types.NewBIP340PubKeyFromBTCPK(covenantPk)
	printRespJSON(
		&covenantKey{
			Name:      ctx.String(keyNameFlag),
			PublicKey: bip340Key.MarshalHex(),
		},
	)

	return err
}

func printRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}
