package main

import (
	"encoding/json"
	"fmt"
	"github.com/babylonchain/btc-validator/config"
	"os"

	"github.com/urfave/cli"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[btc-validator] %v\n", err)
	os.Exit(1)
}

func printRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}

const (
	dbTypeFlag = "db-type"
	dbNameFlag = "db-name"
)

func main() {
	app := cli.NewApp()
	app.Name = "valcli"
	app.Usage = "Control plane for the Bitcoin Validator Daemon (vald)."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  dbTypeFlag,
			Usage: "The type of the database",
			Value: config.DefaultBackend,
		},
		cli.StringFlag{
			Name:  dbNameFlag,
			Usage: "The name of the database bucket",
			Value: config.DefaultDBName,
		},
	}

	app.Commands = append(app.Commands,
		getDaemonInfoCmd,
		createValDaemonCmd,
		lsValDaemonCmd,
		valInfoDaemonCmd,
		registerValDaemonCmd,
		addFinalitySigDaemonCmd,
	)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
