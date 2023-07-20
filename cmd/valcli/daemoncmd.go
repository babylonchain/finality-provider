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
		Usage:     "More advanced commands which require validator daemon to be running.",
		Category:  "Daemon commands",
		Subcommands: []cli.Command{
			getDaemonInfoCmd,
			createValCmd,
			lsValCmd,
		},
	},
}

const (
	valdDaemonAddressFlag = "daemon-address"
)

var (
	defaultValdDaemonAddress = "127.0.0.1:" + strconv.Itoa(valcfg.DefaultRPCPort)
)

var getDaemonInfoCmd = cli.Command{
	Name:      "get-info",
	ShortName: "gi",
	Usage:     "Get information of the running daemon.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  valdDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValdDaemonAddress,
		},
	},
	Action: getInfo,
}

func getInfo(ctx *cli.Context) error {
	daemonAddress := ctx.String(valdDaemonAddressFlag)
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

var createValCmd = cli.Command{
	Name:      "create-validator",
	ShortName: "cv",
	Usage:     "Create a Bitcoin validator object and save it in database.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  valdDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValdDaemonAddress,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Usage:    "The unique name of the validator key",
			Required: true,
		},
	},
	Action: createValDaemon,
}

func createValDaemon(ctx *cli.Context) error {
	daemonAddress := ctx.String(valdDaemonAddressFlag)
	keyName := ctx.String(keyNameFlag)
	client, cleanUp, err := dc.NewValidatorServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	info, err := client.CreateValidator(context.Background(), keyName)

	if err != nil {
		return err
	}

	printRespJSON(info)

	return nil
}

var lsValCmd = cli.Command{
	Name:      "list-validators",
	ShortName: "ls",
	Usage:     "List validators stored in the database.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  valdDaemonAddressFlag,
			Usage: "Full address of the validator daemon in format tcp://<host>:<port>",
			Value: defaultValdDaemonAddress,
		},
	},
	Action: lsValDaemon,
}

func lsValDaemon(ctx *cli.Context) error {
	daemonAddress := ctx.String(valdDaemonAddressFlag)
	rpcClient, cleanUp, err := dc.NewValidatorServiceGRpcClient(daemonAddress)
	if err != nil {
		return err
	}
	defer cleanUp()

	resp, err := rpcClient.QueryValidatorList(context.Background())
	if err != nil {
		return err
	}

	printRespJSON(resp)

	return nil
}
