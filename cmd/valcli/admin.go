package main

import (
	"fmt"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"
)

var adminCommands = []cli.Command{
	{
		Name:      "admin",
		ShortName: "ad",
		Usage:     "Different utility and admin commands",
		Category:  "Admin",
		Subcommands: []cli.Command{
			dumpCfgCommand,
		},
	},
}

const (
	configFileDirFlag = "config-file-dir"
)

var (
	defaultConfigPath = valcfg.DefaultConfigFile
)

var dumpCfgCommand = cli.Command{
	Name:      "dump-config",
	ShortName: "dc",
	Usage:     "Dump default configuration file.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  configFileDirFlag,
			Usage: "Path to where the default config file will be dumped",
			Value: defaultConfigPath,
		},
	},
	Action: dumpCfg,
}

func dumpCfg(c *cli.Context) error {
	configPath := c.String(configFileDirFlag)

	if valcfg.FileExists(configPath) {
		return cli.NewExitError(
			fmt.Sprintf("config already exists under provided path: %s", configPath),
			1,
		)
	}

	defaultConfig := valcfg.DefaultConfig()
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	err := flags.NewIniParser(fileParser).WriteFile(configPath, flags.IniIncludeComments|flags.IniIncludeDefaults)

	if err != nil {
		return err
	}

	return nil
}
