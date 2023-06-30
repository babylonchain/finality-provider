package main

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/urfave/cli"
)

var validatorsCommands = []cli.Command{
	{
		Name:      "validators",
		ShortName: "vals",
		Usage:     "Control BTC validators.",
		Category:  "Validators",
		Subcommands: []cli.Command{
			createValidator, importValidator, registerValidator,
		},
	},
}

const (
	secretFlag = "seed"
)

var createValidator = cli.Command{
	Name:      "create-val",
	ShortName: "cv",
	Usage:     "create a BTC val object using local BTC and Babylon keyrings",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  secretFlag,
			Usage: "Use the given secret to generate the Babylon private key",
		},
	},
	Action: createVal,
}

func createVal(ctx *cli.Context) error {
	var (
		bbnPrivKey *secp256k1.PrivKey
		btcPrivKey *btcec.PrivateKey
	)

	if ctx.IsSet(secretFlag) {
		secret, err := hex.DecodeString(ctx.String(secretFlag))
		if err != nil {
			return fmt.Errorf("failed to generate the Babylon private key: %w", err)
		}
		bbnPrivKey = secp256k1.GenPrivKeyFromSecret(secret)
	}

	bbnPrivKey = secp256k1.GenPrivKey()

	btcPrivKey, err := btcec.NewPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate the BTC private key: %w", err)
	}

	return nil
}

var importValidator = cli.Command{
	Name:      "import-val",
	ShortName: "iv",
	Usage:     "import a BTC val object with given BTC and Babylon addresses",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: importVal,
}

func importVal(ctx *cli.Context) error {
	panic("implement me")
}

var registerValidator = cli.Command{
	Name:      "register-val",
	ShortName: "rv",
	Usage:     "register a existed BTC val to Babylon",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: registerVal,
}

func registerVal(ctx *cli.Context) error {
	panic("implement me")
}
