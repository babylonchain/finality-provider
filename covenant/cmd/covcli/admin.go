package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"

	covcfg "github.com/babylonchain/btc-validator/covenant/config"
)

const configFileFlag = "config"

var adminCommands = []cli.Command{
	{
		Name:      "admin",
		ShortName: "ad",
		Usage:     "Different utility and admin commands.",
		Category:  "Admin",
		Subcommands: []cli.Command{
			dumpCfgCommand,
		},
	},
}

var dumpCfgCommand = cli.Command{
	Name:      "dump-config",
	ShortName: "dc",
	Usage:     "Dump default configuration file.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  configFileFlag,
			Usage: "Path to where the default config file will be dumped",
			Value: covcfg.DefaultConfigFile,
		},
	},
	Action: dumpCfg,
}

func dumpCfg(c *cli.Context) error {
	configPath := c.String(configFileFlag)

	if covcfg.FileExists(configPath) {
		return fmt.Errorf("config already exists under provided path: %s", configPath)
	}

	// ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	defaultConfig := covcfg.DefaultConfig()
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	err := flags.NewIniParser(fileParser).WriteFile(configPath, flags.IniIncludeComments|flags.IniIncludeDefaults)

	if err != nil {
		return err
	}

	return nil
}
