package main

import (
	"encoding/json"
	"fmt"
	"github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/covenant"
	covcfg "github.com/babylonchain/finality-provider/covenant/config"
	"github.com/urfave/cli"
)

type covenantKey struct {
	Name      string `json:"name"`
	PublicKey string `json:"public-key"`
}

var createKeyCommand = cli.Command{
	Name:      "create-key",
	ShortName: "ck",
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
			Name:  homeFlag,
			Usage: "The home directory for the covenant",
			Value: covcfg.DefaultCovenantDir,
		},
	},
	Action: createKey,
}

func createKey(ctx *cli.Context) error {
	homePath := ctx.String(homeFlag)
	chainID := ctx.String(chainIdFlag)
	keyName := ctx.String(keyNameFlag)
	backend := ctx.String(keyringBackendFlag)
	passphrase := ctx.String(passphraseFlag)
	hdPath := ctx.String(hdPathFlag)

	keyPair, err := covenant.CreateCovenantKey(
		homePath,
		chainID,
		keyName,
		backend,
		passphrase,
		hdPath,
	)

	if err != nil {
		return fmt.Errorf("failed to create covenant key: %w", err)
	}

	bip340Key := types.NewBIP340PubKeyFromBTCPK(keyPair.PublicKey)
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
