package e2etest

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/val"
)

func baseEOTSDir() (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, "zEOTSTest")
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}

type eotsManager struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func newEOTSManagerServer(dataDir string, cmd *exec.Cmd) *eotsManager {
	return &eotsManager{
		dataDir: dataDir,
		cmd:     cmd,
	}
}

func (n *eotsManager) start() error {
	if err := n.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(n.dataDir,
		fmt.Sprintf("%s.pid", "config")))
	if err != nil {
		return err
	}

	n.pidFile = pid.Name()
	if _, err = fmt.Fprintf(pid, "%d\n", n.cmd.Process.Pid); err != nil {
		return err
	}

	if err := pid.Close(); err != nil {
		return err
	}

	return nil
}

func (n *eotsManager) stop() (err error) {
	if n.cmd == nil || n.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = n.cmd.Wait()
	}()

	if runtime.GOOS == "windows" {
		return n.cmd.Process.Signal(os.Kill)
	}
	return n.cmd.Process.Signal(os.Interrupt)
}

func (n *eotsManager) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile,
				err)
		}
	}

	dirs := []string{
		n.dataDir,
	}
	var err error
	for _, dir := range dirs {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("Cannot remove dir %s: %v", dir, err)
		}
	}
	return nil
}

func (n *eotsManager) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

type EOTSManagerHandler struct {
	eotsManager *eotsManager
}

func NewEOTSManagerHandler(t *testing.T) *EOTSManagerHandler {
	testDir, err := baseEOTSDir()
	require.NoError(t, err)
	defer func() {
		if err != nil {
			err := os.RemoveAll(testDir)
			require.NoError(t, err)
		}
	}()

	nodeDataDir := filepath.Join(testDir, "eots")

	startEOTSManagerCmd := exec.Command(
		"",
		"testnet",
		"--v=1",
		fmt.Sprintf("--output-dir=%s", testDir),
		"--starting-ip-address=192.168.10.2",
		"--keyring-backend=test",
		"--chain-id=chain-test",
		"--additional-sender-account",
		fmt.Sprintf("--slashing-address=%s", slashingAddr),
		fmt.Sprintf("--jury-pk=%s", juryPkBip340.MarshalHex()),
	)

	var stderr bytes.Buffer
	initTestnetCmd.Stderr = &stderr

	err = initTestnetCmd.Run()
	if err != nil {
		fmt.Printf("init testnet failed: %s \n", stderr.String())
	}
	require.NoError(t, err)

	f, err := os.Create(filepath.Join(testDir, "babylon.log"))
	require.NoError(t, err)

	startCmd := exec.Command(
		"babylond",
		"start",
		fmt.Sprintf("--home=%s", nodeDataDir),
		"--log_level=debug",
	)

	startCmd.Stdout = f

	return &BabylonNodeHandler{
		babylonNode: newBabylonNode(testDir, startCmd, juryKeyName, chainID, slashingAddr),
	}
}

func (w *BabylonNodeHandler) Start() error {
	if err := w.babylonNode.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.babylonNode.cleanup()
		return err
	}
	return nil
}

func (w *BabylonNodeHandler) Stop() error {
	if err := w.babylonNode.shutdown(); err != nil {
		return err
	}

	return nil
}

func (w *BabylonNodeHandler) GetNodeDataDir() string {
	dir := filepath.Join(w.babylonNode.dataDir, "node0", "babylond")
	return dir
}

func (w *BabylonNodeHandler) GetJuryKeyName() string {
	return val.GetKeyName(w.babylonNode.chainID, w.babylonNode.juryKeyName)
}

func (w *BabylonNodeHandler) GetSlashingAddress() string {
	return w.babylonNode.slashingAddr
}
