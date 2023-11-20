package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli"

	"github.com/babylonchain/btc-validator/valcfg"
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
	dbPathFlag = "db-path"
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
			Value: valcfg.DefaultBackend,
		},
		cli.StringFlag{
			Name:  dbPathFlag,
			Usage: "The path of the database file",
			Value: valcfg.DefaultDBPath,
		},
		cli.StringFlag{
			Name:  dbNameFlag,
			Usage: "The name of the database bucket",
			Value: valcfg.DefaultDBName,
		},
	}

	app.Commands = append(app.Commands, daemonCommands...)
	app.Commands = append(app.Commands, adminCommands...)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
