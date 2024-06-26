package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/go-bip39"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
)

type KeyOutput struct {
	Name     string `json:"name" yaml:"name"`
	Address  string `json:"address" yaml:"address"`
	Mnemonic string `json:"mnemonic,omitempty" yaml:"mnemonic"`
}

var KeysCommands = []cli.Command{
	{
		Name:     "keys",
		Usage:    "Command sets of managing keys for interacting with the consumer chain.",
		Category: "Key management",
		Subcommands: []cli.Command{
			AddKeyCmd,
		},
	},
}

var AddKeyCmd = cli.Command{
	Name:  "add",
	Usage: "Add a key to the consumer chain's keyring. Note that this will change the config file in place.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "Path to the keyring directory",
			Value: fpcfg.DefaultFpdDir,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Required: true,
		},
		cli.StringFlag{
			Name:     chainIdFlag,
			Usage:    "The identifier of the consumer chain",
			Required: true,
		},
		cli.StringFlag{
			Name:  keyringBackendFlag,
			Usage: "Select keyring's backend",
			Value: defaultKeyringBackend,
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
		cli.BoolFlag{
			Name:  recoverFlag,
			Usage: "Provide seed phrase to recover existing key instead of creating",
		},
	},
	Action: addKey,
}

func addKey(ctx *cli.Context) error {
	homePath := ctx.String(homeFlag)
	chainID := ctx.String(chainIdFlag)
	keyName := ctx.String(keyNameFlag)
	backend := ctx.String(keyringBackendFlag)
	passphrase := ctx.String(passphraseFlag)
	hdPath := ctx.String(hdPathFlag)
	keyBackend := ctx.String(keyringBackendFlag)

	var (
		mnemonic string
		err      error
	)

	if ctx.Bool(recoverFlag) {
		reader := bufio.NewReader(os.Stdin)
		mnemonic, err = input.GetString("Enter your mnemonic", reader)
		if err != nil {
			return fmt.Errorf("failed to read mnemonic from stdin: %w", err)
		}
		if !bip39.IsMnemonicValid(mnemonic) {
			return errors.New("invalid mnemonic")
		}
	}

	keyInfo, err := service.CreateChainKey(
		homePath,
		chainID,
		keyName,
		backend,
		passphrase,
		hdPath,
		mnemonic,
	)
	if err != nil {
		return fmt.Errorf("failed to create the chain key: %w", err)
	}

	printRespJSON(
		KeyOutput{
			Name:     keyName,
			Address:  keyInfo.AccAddress.String(),
			Mnemonic: keyInfo.Mnemonic,
		},
	)

	// check the config file exists
	cfg, err := fpcfg.LoadConfig(homePath)
	if err != nil {
		return nil // config does not exist, so does not update it
	}

	// write the updated config into the config file
	cfg.BabylonConfig.Key = keyName
	cfg.BabylonConfig.KeyringBackend = keyBackend
	fileParser := flags.NewParser(cfg, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(fpcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}

func printRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("New key for the consumer chain is created "+
		"(mnemonic should be kept in a safe place for recovery):\n%s\n", jsonBytes)
}
