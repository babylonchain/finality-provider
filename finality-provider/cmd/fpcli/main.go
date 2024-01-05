package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/urfave/cli"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
)

var (
	defaultFpdDaemonAddress = "127.0.0.1:" + strconv.Itoa(fpcfg.DefaultRPCPort)
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[fpd] %v\n", err)
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

func main() {
	app := cli.NewApp()
	app.Name = "fpcli"
	app.Usage = "Control plane for the Finality Provider Daemon (fpd)."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  fpdDaemonAddressFlag,
			Usage: "The RPC server address of fpd",
			Value: defaultFpdDaemonAddress,
		},
	}

	app.Commands = append(app.Commands,
		getDaemonInfoCmd,
		createFpDaemonCmd,
		lsFpDaemonCmd,
		fpInfoDaemonCmd,
		registerFpDaemonCmd,
		addFinalitySigDaemonCmd,
	)

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}
