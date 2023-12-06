package main

import (
	"fmt"
	covcfg "github.com/babylonchain/btc-validator/covenant/config"
	"github.com/babylonchain/btc-validator/util"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"
	"path/filepath"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Initialize a covenant home directory.",
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
	homePath, err := filepath.Abs(c.String(homeFlag))
	if err != nil {
		return err
	}
	force := c.Bool(forceFlag)

	if util.FileExists(homePath) && !force {
		return fmt.Errorf("home path %s already exists", homePath)
	}

	// ensure the directory exists
	homePath = util.CleanAndExpandPath(homePath)
	if err := util.MakeDirectory(homePath); err != nil {
		return err
	}
	// Create log directory
	logDir := covcfg.LogDir(homePath)
	if err := util.MakeDirectory(logDir); err != nil {
		return err
	}

	defaultConfig := covcfg.DefaultConfigWithHomePath(homePath)
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(covcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
