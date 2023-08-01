package main

import (
	"encoding/hex"
	"fmt"

	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	chainIdFlag           = "chain-id"
	keyringDirFlag        = "keyring-dir"
	keyringBackendFlag    = "keyring-backend"
	defaultChainID        = "chain-test"
	defaultKeyringBackend = "test"
)

var validatorsCommands = []cli.Command{
	{
		Name:      "validators",
		ShortName: "vals",
		Usage:     "Control Bitcoin validators.",
		Category:  "Validators",
		Subcommands: []cli.Command{
			createValCmd, listValCmd, importValCmd,
		},
	},
}

var createValCmd = cli.Command{
	Name:      "create-validator",
	ShortName: "cv",
	Usage:     "Create a Bitcoin validator object and save it in database.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  chainIdFlag,
			Usage: "The chainID of the Babylonchain",
			Value: defaultChainID,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Usage:    "The unique name of the validator key",
			Required: true,
		},
		cli.StringFlag{
			Name:  keyringBackendFlag,
			Usage: "Select keyring's backend (os|file|test)",
			Value: defaultKeyringBackend,
		},
		cli.StringFlag{
			Name:  keyringDirFlag,
			Usage: "The directory where the keyring is stored",
		},
	},
	Action: createVal,
}

func createVal(ctx *cli.Context) error {
	sdkCtx, err := service.CreateClientCtx(
		ctx.String(keyringDirFlag),
		ctx.String(chainIdFlag),
	)
	if err != nil {
		return err
	}

	krController, err := val.NewKeyringController(
		sdkCtx,
		ctx.String(keyNameFlag),
		ctx.String(keyringBackendFlag),
	)
	if err != nil {
		return err
	}

	if krController.ValidatorKeyNameTaken() {
		return fmt.Errorf("the key name %s is taken", krController.GetKeyName())
	}

	validator, err := krController.CreateBTCValidator()
	if err != nil {
		return err
	}

	vs, err := getValStoreFromCtx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = vs.Close()
	}()

	if err := vs.SaveValidator(validator); err != nil {
		return err
	}

	printRespJSON(&proto.CreateValidatorResponse{
		BabylonPk: hex.EncodeToString(validator.BabylonPk),
		BtcPk:     hex.EncodeToString(validator.BtcPk),
	})

	return err
}

var listValCmd = cli.Command{
	Name:      "list-validators",
	ShortName: "ls",
	Usage:     "List validators stored in the database.",
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

	valsInfo := make([]*proto.ValidatorInfo, len(valList))
	for i, v := range valList {
		valInfo := proto.NewValidatorInfo(v)
		valsInfo[i] = valInfo
	}

	printRespJSON(&proto.QueryValidatorListResponse{Validators: valsInfo})

	return nil
}

var importValCmd = cli.Command{
	Name:      "import-validator",
	ShortName: "iv",
	Usage:     "Import a Bitcoin validator object with given Bitcoin and Babylon addresses.",
	Flags:     []cli.Flag{
		// TODO: add flags
	},
	Action: importVal,
}

func importVal(ctx *cli.Context) error {
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
