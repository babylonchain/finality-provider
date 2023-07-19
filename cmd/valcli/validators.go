package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/service"
	dc "github.com/babylonchain/btc-validator/service/client"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	chainIdFlag           = "chain-id"
	keyringDirFlag        = "keyring-dir"
	keyringBackendFlag    = "keyring-backend"
	keyNameFlag           = "key-name"
	randNumFlag           = "rand-num"
	babylonPkFlag         = "babylon-pk"
	defaultChainID        = "test-chain"
	defaultKeyringBackend = "test"
	defaultRandomNum      = 100
)

var validatorsCommands = []cli.Command{
	{
		Name:      "validators",
		ShortName: "vals",
		Usage:     "Control Bitcoin validators.",
		Category:  "Validators",
		Subcommands: []cli.Command{
			createValidator, listValidators, importValidator, registerValidator, commitRandomList,
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
		BabylonPk: hex.EncodeToString(validator.BabylonPk),
		BtcPk:     hex.EncodeToString(validator.BtcPk),
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
			Name:  valdDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValdDaemonAddress,
		},
	},
	Action: registerVal,
}

func registerVal(ctx *cli.Context) error {
	pkBytes := []byte(ctx.Args().First())
	daemonAddress := ctx.String(valdDaemonAddressFlag)
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

// TODO: consider remove this command after PoC
// because leaving this command to users is dangerous
// committing random list should be an automatic process
var commitRandomList = cli.Command{
	Name:      "commit-random-list",
	ShortName: "crl",
	Usage:     "Generate a list of Schnorr random pair and commit the public rand for Bitcoin validator.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  valdDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValdDaemonAddress,
		},
		cli.Int64Flag{
			Name:  randNumFlag,
			Usage: "The number of public randomness you want to commit",
			Value: int64(defaultRandomNum),
		},
		cli.StringFlag{
			Name:  babylonPkFlag,
			Usage: "Commit random list for a specific Bitcoin validator",
		},
	},
	Action: commitRand,
}

func commitRand(ctx *cli.Context) error {
	daemonAddress := ctx.String(valdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewValidatorServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	var bbnPkBytes []byte
	if ctx.String(babylonPkFlag) != "" {
		bbnPkBytes = []byte(ctx.String(babylonPkFlag))
	}
	res, err := rpcClient.CommitPubRandList(context.Background(),
		bbnPkBytes)
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
