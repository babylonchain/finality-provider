package main

import (
	"encoding/json"
	"fmt"

	"github.com/babylonchain/babylon/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/covenant"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
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
	keyringDir := ctx.String(keyringDirFlag)
	chainID := ctx.String(chainIdFlag)
	keyName := ctx.String(keyNameFlag)
	backend := ctx.String(keyringBackendFlag)
	passphrase := ctx.String(passphraseFlag)
	hdPath := ctx.String(hdPathFlag)

	keyPair, err := covenant.CreateCovenantKey(
		keyringDir,
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
