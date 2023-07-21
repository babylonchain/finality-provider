package main

import (
	"encoding/hex"
	"fmt"

	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/val"
)

type juryKey struct {
	Name      string `json:"name"`
	PublicKey string `json:"public-key"`
}

var juryCommands = []cli.Command{
	{
		Name:      "jury",
		ShortName: "j",
		Usage:     "Control Babylon Jury.",
		Category:  "Jury",
		Subcommands: []cli.Command{
			createJury,
		},
	},
}

var createJury = cli.Command{
	Name:      "create-jury",
	ShortName: "cj",
	Usage:     "Create a Jury account in the keyring.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  chainIdFlag,
			Usage: "The chainID of the Babylonchain",
			Value: defaultChainID,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Usage:    "The unique name of the Jury key",
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
	Action: createJuryKey,
}

func createJuryKey(ctx *cli.Context) error {
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

	if krController.JuryKeyTaken() {
		return fmt.Errorf("the Jury key name %s is taken", krController.GetKeyName())
	}

	juryPk, err := krController.CreateJuryKey()
	if err != nil {
		return fmt.Errorf("failed to create Jury key: %w", err)
	}

	printRespJSON(&juryKey{
		Name:      krController.GetKeyName(),
		PublicKey: hex.EncodeToString(juryPk.SerializeCompressed()),
	})

	return err
}
