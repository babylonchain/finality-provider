package main

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

var validatorsCommands = []cli.Command{
	{
		Name:      "validators",
		ShortName: "vals",
		Usage:     "Control BTC validators.",
		Category:  "Validators",
		Subcommands: []cli.Command{
			createValidator, listValidators, importValidator, registerValidator,
		},
	},
}

const (
	bbnPrivKeyFlag = "babylon-priv-key"
	btcPrivKeyFlag = "btc-priv-key"
)

var createValidator = cli.Command{
	Name:      "create-validator",
	ShortName: "cv",
	Usage:     "create a BTC validator object using local BTC and Babylon keyrings",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  bbnPrivKeyFlag,
			Usage: "Use the given Babylon private key to generate the validator",
		},
		cli.StringFlag{
			Name:  btcPrivKeyFlag,
			Usage: "Use the given BTC private key to generate the validator",
		},
	},
	Action: createVal,
}

func createVal(ctx *cli.Context) error {
	var (
		bbnPrivKey *btcec.PrivateKey
		btcPrivKey *btcec.PrivateKey

		err error
	)

	if ctx.IsSet(bbnPrivKeyFlag) {
		bbnPrivKeyBytes, err := hex.DecodeString(ctx.String(bbnPrivKeyFlag))
		if err != nil {
			return fmt.Errorf("failed to decode the Babylon private key from the given bytes: %w", err)
		}
		bbnPrivKey, _ = btcec.PrivKeyFromBytes(bbnPrivKeyBytes)
	} else {
		bbnPrivKey, err = btcec.NewPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate Babylon private key: %w", err)
		}
	}

	if ctx.IsSet(btcPrivKeyFlag) {
		btcPrivKeyBytes, err := hex.DecodeString(ctx.String(btcPrivKeyFlag))
		if err != nil {
			return fmt.Errorf("failed to decode the Babylon private key from the given bytes: %w", err)
		}
		btcPrivKey, _ = btcec.PrivKeyFromBytes(btcPrivKeyBytes)
	} else {
		btcPrivKey, err = btcec.NewPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate Babylon private key: %w", err)
		}
	}

	validator := val.CreateValidator(bbnPrivKey.PubKey(), btcPrivKey.PubKey())

	dbcfg := &valcfg.DatabaseConfig{
		DbType: ctx.GlobalString(dbTypeFlag),
		Path:   ctx.GlobalString(dbPathFlag),
		Name:   ctx.GlobalString(dbNameFlag),
	}
	s, err := openStore(dbcfg)
	defer closeStore(s)
	if err != nil {
		return fmt.Errorf("failed to open the database: %w", err)
	}

	err = val.SaveValidator(s, validator)
	if err != nil {
		return fmt.Errorf("failed to save the created validator to the database: %w", err)
	}

	fmt.Printf("A new BTC validator is created and stored in the database!\n"+
		"Babylon public key: %x\n"+
		"BTC public key: %x\n",
		validator.BabylonPk, validator.BtcPk)

	return nil
}

var listValidators = cli.Command{
	Name:      "list-validators",
	ShortName: "ls",
	Usage:     "list validators stored in the database",
	Action:    lsVal,
}

func lsVal(ctx *cli.Context) error {
	dbcfg := &valcfg.DatabaseConfig{
		DbType: ctx.GlobalString(dbTypeFlag),
		Path:   ctx.GlobalString(dbPathFlag),
		Name:   ctx.GlobalString(dbNameFlag),
	}
	s, err := openStore(dbcfg)
	defer closeStore(s)
	if err != nil {
		return fmt.Errorf("failed to open the database: %w", err)
	}

	valsList, err := val.ListValidators(s)
	if err != nil {
		return err
	}

	printRespJSON(valsList)

	return nil
}

var importValidator = cli.Command{
	Name:      "import-validator",
	ShortName: "iv",
	Usage:     "import a BTC validator object with given BTC and Babylon addresses",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: importVal,
}

func importVal(ctx *cli.Context) error {
	panic("implement me")
}

var registerValidator = cli.Command{
	Name:      "register-validator",
	ShortName: "rv",
	Usage:     "register a existed BTC validator to Babylon",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: registerVal,
}

func registerVal(ctx *cli.Context) error {
	panic("implement me")
}
