package main

import (
	"context"
	"strconv"

	"github.com/urfave/cli"

	dc "github.com/babylonchain/btc-validator/service/client"
	"github.com/babylonchain/btc-validator/valcfg"
)

var daemonCommands = []cli.Command{
	{
		Name:      "daemon",
		ShortName: "dn",
		Usage:     "More advanced commands which require validator daemon to be running",
		Category:  "Daemon commands",
		Subcommands: []cli.Command{
			getDaemonInfo,
		},
	},
}

const (
	validatorDaemonAddressFlag = "daemon-address"
)

var (
	defaultValidatorDaemonAddress = "127.0.0.1:" + strconv.Itoa(valcfg.DefaultRPCPort)
)

var getDaemonInfo = cli.Command{
	Name:      "get-info",
	ShortName: "gi",
	Usage:     "Get information of the running daemon",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  validatorDaemonAddressFlag,
			Usage: "full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValidatorDaemonAddress,
		},
	},
	Action: getInfo,
}

func getInfo(ctx *cli.Context) error {
	daemonAddress := ctx.String(validatorDaemonAddressFlag)
	client, cleanUp, err := dc.NewValidatorServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	info, err := client.GetInfo(context.Background())

	if err != nil {
		return err
	}

	printRespJSON(info)

	return nil
}
