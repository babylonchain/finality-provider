package main

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/codec"
	"github.com/babylonchain/btc-validator/proto"
	dc "github.com/babylonchain/btc-validator/service/client"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	chainIdFlag        = "chain-id"
	keyringDirFlag     = "keyring-dir"
	keyringBackendFlag = "keyring-backend"
	keyNameFlag        = "key-name"

	defaultChainID        = "test-chain"
	defaultKeyringBackend = "test"
)

var validatorsCommands = []cli.Command{
	{
		Name:      "validators",
		ShortName: "vals",
		Usage:     "Control Bitcoin validators.",
		Category:  "Validators",
		Subcommands: []cli.Command{
			createValidator, listValidators, importValidator, registerValidator,
		},
	},
}

var createValidator = cli.Command{
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
	sdkCtx, err := createClientCtx(ctx)
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

	if krController.KeyNameTaken() {
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
		BabylonPk: validator.BabylonPk,
		BtcPk:     validator.BtcPk,
	})

	return err
}

var listValidators = cli.Command{
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

	printRespJSON(&proto.QueryValidatorListResponse{Validators: valList})

	return nil
}

var importValidator = cli.Command{
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

var registerValidator = cli.Command{
	Name:      "register-validator",
	ShortName: "rv",
	Usage:     "Register a created Bitcoin validator to Babylon, requiring the validator daemon running.",
	UsageText: "register-validator [Babylon public key]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  validatorDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValidatorDaemonAddress,
		},
	},
	Action: registerVal,
}

func registerVal(ctx *cli.Context) error {
	pkBytes := []byte(ctx.Args().First())
	daemonAddress := ctx.String(validatorDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewValidatorServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	res, err := rpcClient.RegisterValidator(context.Background(), pkBytes)
	if err != nil {
		return err
	}

	printRespJSON(res)

	return nil
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

func createClientCtx(ctx *cli.Context) (client.Context, error) {
	dir := ctx.String(keyringDirFlag)
	if dir == "" {
		dir = valcfg.DefaultValidatordDir
	}

	return client.Context{}.
		WithChainID(ctx.String(chainIdFlag)).
		WithCodec(codec.MakeCodec()).
		WithKeyringDir(dir), nil
}
