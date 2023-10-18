package main

import (
	"fmt"

	"github.com/babylonchain/babylon/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"

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

	krController, err := val.NewChainKeyringController(
		sdkCtx,
		ctx.String(keyNameFlag),
		ctx.String(keyringBackendFlag),
	)
	if err != nil {
		return err
	}

	sdkJuryPk, err := krController.CreateChainKey()
	if err != nil {
		return fmt.Errorf("failed to create Jury key: %w", err)
	}
	juryPk, err := secp256k1.ParsePubKey(sdkJuryPk.Key)
	if err != nil {
		return err
	}

	bip340Key := types.NewBIP340PubKeyFromBTCPK(juryPk)
	printRespJSON(&juryKey{
		Name:      ctx.String(keyNameFlag),
		PublicKey: bip340Key.MarshalHex(),
	})

	return err
}
