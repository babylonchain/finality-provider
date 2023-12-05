package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"

	covcfg "github.com/babylonchain/btc-validator/covenant/config"
)

const (
	homeFlag  = "home"
	forceFlag = "force"
)

var adminCommands = []cli.Command{
	{
		Name:      "admin",
		ShortName: "ad",
		Usage:     "Different utility and admin commands.",
		Category:  "Admin",
		Subcommands: []cli.Command{
			initCommand,
		},
	},
}

var initCommand = cli.Command{
	Name:      "init",
	ShortName: "in",
	Usage:     "Initialize a covenant home directory.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "Path to where the home directory will be initialized",
			Value: covcfg.DefaultCovenantDir,
		},
		cli.BoolFlag{
			Name:     forceFlag,
			Usage:    "Override existing configuration",
			Required: false,
		},
	},
	Action: initHome,
}

func initHome(c *cli.Context) error {
	homePath := c.String(homeFlag)
	force := c.Bool(forceFlag)

	_, err := os.Stat(homePath)
	if os.IsExist(err) {
		if !force {
			return fmt.Errorf("home path %s already exists", homePath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// ensure the directory exists
	configDir := filepath.Dir(homePath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	defaultConfig := covcfg.DefaultConfigWithHomePath(homePath)
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(covcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
