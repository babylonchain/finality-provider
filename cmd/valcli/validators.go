package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/go-bip39"
	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"
)

const (
	keyringBackendFlag = "keyring-backend"
	keyNameFlag        = "key-name"

	secp256k1Type       = "secp256k1"
	btcPrefix           = "btc-"
	babylonPrefix       = "bbn-"
	mnemonicEntropySize = 256
)

var (
	defaultKeyringBackend = "test"
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
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     keyNameFlag,
			Usage:    "the unique name of the validator key",
			Required: true,
		},
		cli.StringFlag{
			Name:  keyringBackendFlag,
			Usage: "select keyring's backend (os|file|test)",
			Value: defaultKeyringBackend,
		},
	},
	Action: createVal,
}

func createVal(ctx *cli.Context) error {
	// TODO add more sdk context fields
	sdkCtx := client.Context{}
	kr, err := createKeyring(sdkCtx, ctx.String(keyringBackendFlag))
	if err != nil {
		return err
	}

	keyName := ctx.String(keyNameFlag)
	bbnName := babylonPrefix + keyName
	btcName := btcPrefix + keyName
	if keyExists(bbnName, kr) || keyExists(btcName, kr) {
		return fmt.Errorf("the key name is taken, please choose another one")
	}

	// create babylon keyring
	babylonPubKey, err := createKey(bbnName, kr)
	if err != nil {
		return err
	}

	// create BTC keyring
	btcPubKey, err := createKey(btcName, kr)
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

	validator := val.NewValidator(babylonPubKey, btcPubKey)
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

func createKeyring(sdkCtx client.Context, keyringBackend string) (keyring.Keyring, error) {
	if keyringBackend == "" {
		return nil, fmt.Errorf("the keyring backend should not be empty")
	}

	return client.NewKeyringFromBackend(sdkCtx, keyringBackend)
}

func createKey(name string, kr keyring.Keyring) (*btcec.PublicKey, error) {
	keyringAlgos, _ := kr.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(secp256k1Type, keyringAlgos)
	if err != nil {
		return nil, err
	}

	// read entropy seed straight from tmcrypto.Rand and convert to mnemonic
	entropySeed, err := bip39.NewEntropy(mnemonicEntropySize)
	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return nil, err
	}

	record, err := kr.NewAccount(name, mnemonic, "", "", algo)
	if err != nil {
		return nil, err
	}

	pkBytes, err := record.GetPubKey()
	if err != nil {
		return nil, err
	}

	pk, err := btcec.ParsePubKey(pkBytes.Bytes())
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func keyExists(name string, kr keyring.Keyring) bool {
	_, err := kr.Key(name)
	return err == nil
}
