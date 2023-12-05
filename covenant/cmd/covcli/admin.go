package main

import (
	"fmt"
	"github.com/babylonchain/btc-validator/util"
	"github.com/jessevdk/go-flags"
	"github.com/urfave/cli"
	"os"
	"path/filepath"

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
	homePath, err := filepath.Abs(c.String(homeFlag))
	if err != nil {
		return err
	}
	force := c.Bool(forceFlag)

	_, err = os.Stat(util.CleanAndExpandPath(homePath))
	if util.FileExists(homePath) && !force {
		return fmt.Errorf("home path %s already exists", homePath)
	}

	// ensure the directory exists
	homeDir := util.CleanAndExpandPath(homePath)
	if err := util.MakeDirectory(homeDir); err != nil {
		return err
	}
	// Create log directory
	logDir := util.CleanAndExpandPath(covcfg.LogDir(homePath))
	if err := util.MakeDirectory(logDir); err != nil {
		return err
	}

	defaultConfig := covcfg.DefaultConfigWithHomePath(homePath)
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(covcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
