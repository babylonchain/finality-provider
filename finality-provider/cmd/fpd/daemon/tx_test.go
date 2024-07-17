package daemon_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	fpcmd "github.com/babylonchain/finality-provider/finality-provider/cmd"
	"github.com/babylonchain/finality-provider/finality-provider/cmd/fpd/daemon"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewValidateSignedFinalityProviderCmd(t *testing.T) {
	root := rootCmd()
	temp := filepath.Join(t.TempDir(), "homefpnew")

	_, _, err := executeCommandC(root, "init", fmt.Sprintf("--home=%s", temp))
	require.NoError(t, err)
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fpd",
		PersistentPreRunE: fpcmd.PersistClientCtx(client.Context{}),
	}
	cmd.PersistentFlags().String(flags.FlagHome, fpcfg.DefaultFpdDir, "The application home directory")

	cmd.AddCommand(
		daemon.CommandInit(), daemon.CommandStart(), daemon.CommandKeys(),
		daemon.CommandGetDaemonInfo(), daemon.CommandCreateFP(), daemon.CommandLsFP(),
		daemon.CommandInfoFP(), daemon.CommandRegisterFP(), daemon.CommandAddFinalitySig(),
		daemon.CommandExportFP(), daemon.CommandTxs(),
	)

	return cmd
}

func executeCommandC(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	c, err = root.ExecuteC()

	return c, buf.String(), err
}
