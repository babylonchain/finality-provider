package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"

	eotscfg "github.com/babylonchain/btc-validator/eotsmanager/config"
)

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

const (
	configFileDirFlag = "config-file-dir"
)

var (
	defaultConfigPath = eotscfg.DefaultConfigFile
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

	if eotscfg.FileExists(configPath) {
		return cli.NewExitError(
			fmt.Sprintf("config already exists under provided path: %s", configPath),
			1,
		)
	}

	// ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	defaultConfig := eotscfg.DefaultConfig()
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	err := flags.NewIniParser(fileParser).WriteFile(configPath, flags.IniIncludeComments|flags.IniIncludeDefaults)

	if err != nil {
		return err
	}

	return nil
}
