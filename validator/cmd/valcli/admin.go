package main

import (
	"fmt"
	"github.com/babylonchain/btc-validator/util"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"
	"path/filepath"

	valcfg "github.com/babylonchain/btc-validator/validator/config"
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

const (
	homeFlag  = "home"
	forceFlag = "force"
)

var initCommand = cli.Command{
	Name:      "init",
	ShortName: "in",
	Usage:     "Initialize a validator home directory",
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
	force := c.Bool(forceFlag)

	if util.FileExists(homePath) && !force {
		return fmt.Errorf("home path %s already exists", homePath)
	}

	// Create home directory
	homeDir := util.CleanAndExpandPath(homePath)
	if err := util.MakeDirectory(homeDir); err != nil {
		return err
	}
	// Create log directory
	logDir := util.CleanAndExpandPath(valcfg.LogDir(homePath))
	if err := util.MakeDirectory(logDir); err != nil {
		return err
	}

	defaultConfig := valcfg.DefaultConfig()
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(valcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
