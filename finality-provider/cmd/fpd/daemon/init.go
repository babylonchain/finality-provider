package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/jessevdk/go-flags"
	"github.com/spf13/cobra"

	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/util"
)

// CommandInit returns the init command of fpd daemon that starts the config dir.
func CommandInit() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "init",
		Short:   "Initialize a finality-provider home directory.",
		Long:    `Creates a new finality-provider home directory with default config`,
		Example: `fpd init --home /home/user/.fpd --force`,
		Args:    cobra.NoArgs,
		RunE:    runInitCmdPrepare,
	}
	cmd.Flags().Bool(forceFlag, false, "Override existing configuration")
	return cmd
}

func runInitCmdPrepare(cmd *cobra.Command, args []string) error {
	clientCtx, err := client.GetClientQueryContext(cmd)
	if err != nil {
		return err
	}

	return runInitCmd(clientCtx, cmd, args)
}

func runInitCmd(ctx client.Context, cmd *cobra.Command, args []string) error {
	homePath, err := filepath.Abs(ctx.HomeDir)
	if err != nil {
		return err
	}

	homePath = util.CleanAndExpandPath(homePath)
	force, err := cmd.Flags().GetBool(forceFlag)
	if err != nil {
		return fmt.Errorf("failed to read flag %s: %w", fpPkFlag, err)
	}

	if util.FileExists(homePath) && !force {
		return fmt.Errorf("home path %s already exists", homePath)
	}

	if err := util.MakeDirectory(homePath); err != nil {
		return err
	}
	// Create log directory
	logDir := fpcfg.LogDir(homePath)
	if err := util.MakeDirectory(logDir); err != nil {
		return err
	}

	defaultConfig := fpcfg.DefaultConfigWithHome(homePath)
	fileParser := flags.NewParser(&defaultConfig, flags.Default)

	return flags.NewIniParser(fileParser).WriteFile(fpcfg.ConfigFile(homePath), flags.IniIncludeComments|flags.IniIncludeDefaults)
}
