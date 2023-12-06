package main

import (
	"fmt"
	"github.com/babylonchain/btc-validator/util"
	valcfg "github.com/babylonchain/btc-validator/validator/config"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"
	"path/filepath"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Initialize a validator home directory",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "Path to where the home directory will be initialized",
			Value: valcfg.DefaultValdDir,
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
	homePath, err := filepath.Abs(c.String(homeFlag))
	if err != nil {
		return err
	}
	// Create home directory
	homePath = util.CleanAndExpandPath(homePath)
	force := c.Bool(forceFlag)

	if util.FileExists(homePath) && !force {
		return fmt.Errorf("home path %s already exists", homePath)
	}

	if err := util.MakeDirectory(homePath); err != nil {
		return err
	}
	// Create log directory
	logDir := valcfg.LogDir(homePath)
	if err := util.MakeDirectory(logDir); err != nil {
		return err
	}

	defaultConfig := valcfg.DefaultConfigWithHome(homePath)
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(valcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
