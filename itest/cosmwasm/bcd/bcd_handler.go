package e2etest_bcd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	common "github.com/babylonchain/finality-provider/itest/common"
	"github.com/stretchr/testify/require"
)

const (
	bcdRpcPort int = 3990
	bcdP2pPort int = 3991
	bcdChainID     = "bcd-test"
)

type BcdNodeHandler struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func NewBcdNodeHandler(t *testing.T) *BcdNodeHandler {
	testDir, err := common.BaseDir("ZBcdTest")
	require.NoError(t, err)
	defer func() {
		if err != nil {
			err := os.RemoveAll(testDir)
			require.NoError(t, err)
		}
	}()

	setupBcd(t, testDir)
	cmd := bcdStartCmd(t, testDir)
	return &BcdNodeHandler{
		cmd:     cmd,
		pidFile: "", // empty for now, will be set after start
		dataDir: testDir,
	}
}

func (w *BcdNodeHandler) Start() error {
	if err := w.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.cleanup()
		return err
	}
	return nil
}

func (w *BcdNodeHandler) Stop(t *testing.T) {
	err := w.stop()
	// require.NoError(t, err)
	// TODO: investigate why stopping the bcd process is failing
	if err != nil {
		log.Printf("error stopping bcd process: %v", err)
	}

	err = w.cleanup()
	require.NoError(t, err)
}

func (w *BcdNodeHandler) GetRpcUrl() string {
	return fmt.Sprintf("http://localhost:%d", bcdRpcPort)
}

func (w *BcdNodeHandler) GetHomeDir() string {
	return w.dataDir
}

func (w *BcdNodeHandler) start() error {
	if err := w.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(w.dataDir, fmt.Sprintf("%s.pid", "bcd")))
	if err != nil {
		return err
	}

	w.pidFile = pid.Name()
	if _, err = fmt.Fprintf(pid, "%d\n", w.cmd.Process.Pid); err != nil {
		return err
	}

	if err := pid.Close(); err != nil {
		return err
	}

	return nil
}

func (w *BcdNodeHandler) stop() (err error) {
	if w.cmd == nil || w.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = w.cmd.Wait()
	}()

	if runtime.GOOS == "windows" {
		return w.cmd.Process.Signal(os.Kill)
	}
	return w.cmd.Process.Signal(os.Interrupt)
}

func (w *BcdNodeHandler) cleanup() error {
	if w.pidFile != "" {
		if err := os.Remove(w.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", w.pidFile, err)
		}
	}

	dirs := []string{
		w.dataDir,
	}
	var err error
	for _, dir := range dirs {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("Cannot remove dir %s: %v", dir, err)
		}
	}
	return nil
}

func bcdInit(homeDir string) error {
	_, err := common.RunCommand("bcd", "init", "--home", homeDir, "--chain-id", bcdChainID, common.WasmMoniker)
	return err
}

func bcdUpdateGenesisFile(homeDir string) error {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	sedCmd := fmt.Sprintf("sed -i. 's/\"WasmStake\"/\"%s\"/' %s", common.WasmStake, genesisPath)
	_, err := common.RunCommand("sh", "-c", sedCmd)
	return err
}

func bcdKeysAdd(homeDir string) error {
	_, err := common.RunCommand("bcd", "keys", "add", "validator", "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdAddValidatorGenesisAccount(homeDir string) error {
	_, err := common.RunCommand("bcd", "genesis", "add-genesis-account", "validator", fmt.Sprintf("1000000000000%s,1000000000000%s", common.WasmStake, common.WasmFee), "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdGentxValidator(homeDir string) error {
	_, err := common.RunCommand("bcd", "genesis", "gentx", "validator", fmt.Sprintf("250000000%s", common.WasmStake), "--chain-id="+bcdChainID, "--amount="+fmt.Sprintf("250000000%s", common.WasmStake), "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdCollectGentxs(homeDir string) error {
	_, err := common.RunCommand("bcd", "genesis", "collect-gentxs", "--home", homeDir)
	return err
}

func setupBcd(t *testing.T, testDir string) {
	err := bcdInit(testDir)
	require.NoError(t, err)

	err = bcdUpdateGenesisFile(testDir)
	require.NoError(t, err)

	err = bcdKeysAdd(testDir)
	require.NoError(t, err)

	err = bcdAddValidatorGenesisAccount(testDir)
	require.NoError(t, err)

	err = bcdGentxValidator(testDir)
	require.NoError(t, err)

	err = bcdCollectGentxs(testDir)
	require.NoError(t, err)
}

func bcdStartCmd(t *testing.T, testDir string) *exec.Cmd {
	args := []string{
		"start",
		"--home", testDir,
		"--rpc.laddr", fmt.Sprintf("tcp://0.0.0.0:%d", bcdRpcPort),
		"--p2p.laddr", fmt.Sprintf("tcp://0.0.0.0:%d", bcdP2pPort),
		"--log_level=info",
		"--trace",
	}

	f, err := os.Create(filepath.Join(testDir, "bcd.log"))
	require.NoError(t, err)

	cmd := exec.Command("bcd", args...)
	cmd.Stdout = f
	cmd.Stderr = f

	return cmd
}
