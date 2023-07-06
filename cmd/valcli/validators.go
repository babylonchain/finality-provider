package main

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"
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

var createValidator = cli.Command{
	Name:      "create-validator",
	ShortName: "cv",
	Usage:     "create a BTC validator object and save it in database",
	Flags:     []cli.Flag{},
	Action:    createVal,
}

func createVal(ctx *cli.Context) error {
	vs, err := getValStoreFromCtx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = vs.Close()
	}()

	bbnPrivKey, btcPrivKey, err := val.GenerateValPrivKeys()
	if err != nil {
		return err
	}

	// TODO secure private keys in a key string

	validator := val.NewValidator(bbnPrivKey.PubKey(), btcPrivKey.PubKey())
	if err := vs.SaveValidator(validator); err != nil {
		return err
	}

	printRespJSON(&valrpc.CreateValidatorResponse{
		BabylonPk: validator.BabylonPk,
		BtcPk:     validator.BtcPk,
	})

	return err
}

var listValidators = cli.Command{
	Name:      "list-validators",
	ShortName: "ls",
	Usage:     "list validators stored in the database",
	Action:    lsVal,
}

func lsVal(ctx *cli.Context) error {
	vs, err := getValStoreFromCtx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = vs.Close()
	}()

	valList, err := vs.ListValidators()
	if err != nil {
		return err
	}

	printRespJSON(&valrpc.QueryValidatorListResponse{Validators: valList})

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
	Usage:     "register a created BTC validator to Babylon",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: registerVal,
}

func registerVal(ctx *cli.Context) error {
	panic("implement me")
}

func getValStoreFromCtx(ctx *cli.Context) (*val.ValidatorStore, error) {
	dbcfg, err := valcfg.NewDatabaseConfig(
		ctx.GlobalString(dbTypeFlag),
		ctx.GlobalString(dbPathFlag),
		ctx.GlobalString(dbNameFlag),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid DB config: %w", err)
	}

	valStore, err := val.NewValidatorStore(dbcfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store: %w", err)
	}

	return valStore, nil
}
