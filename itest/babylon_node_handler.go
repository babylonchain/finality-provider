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

	"github.com/99designs/keyring"
	"github.com/babylonchain/babylon/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/service"
	"github.com/babylonchain/btc-validator/val"
)

type babylonNode struct {
	cmd             *exec.Cmd
	pidFile         string
	dataDir         string
	covenantKeyName string
	chainID         string
	slashingAddr    string
}

func newBabylonNode(dataDir string, cmd *exec.Cmd, covenantKeyName, chainID string, slashingAddr string) *babylonNode {
	return &babylonNode{
		dataDir:         dataDir,
		cmd:             cmd,
		covenantKeyName: covenantKeyName,
		chainID:         chainID,
		slashingAddr:    slashingAddr,
	}
}

func (n *babylonNode) start() error {
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

func (n *babylonNode) stop() (err error) {
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

func (n *babylonNode) cleanup() error {
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

func (n *babylonNode) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

type BabylonNodeHandler struct {
	babylonNode *babylonNode
}

func NewBabylonNodeHandler(t *testing.T) *BabylonNodeHandler {
	testDir, err := baseDir("zBabylonTest")
	require.NoError(t, err)
	defer func() {
		if err != nil {
			err := os.RemoveAll(testDir)
			require.NoError(t, err)
		}
	}()

	nodeDataDir := filepath.Join(testDir, "node0", "babylond")

	// the Covenant key needs to be created before babylond is started
	sdkCtx, err := service.CreateClientCtx(
		nodeDataDir,
		chainID,
	)
	require.NoError(t, err)
	covenantKeyName := "covenant-key"
	krController, err := val.NewChainKeyringController(
		sdkCtx,
		covenantKeyName,
		string(keyring.FileBackend),
	)
	require.NoError(t, err)
	sdkCovenantPk, err := krController.CreateChainKey(passphrase, hdPath)
	require.NoError(t, err)
	covenantPk, err := secp256k1.ParsePubKey(sdkCovenantPk.Key)
	require.NoError(t, err)
	covenantPkBip340 := types.NewBIP340PubKeyFromBTCPK(covenantPk)

	slashingAddr := "SZtRT4BySL3o4efdGLh3k7Kny8GAnsBrSW"

	initTestnetCmd := exec.Command(
		"babylond",
		"testnet",
		"--v=1",
		fmt.Sprintf("--output-dir=%s", testDir),
		"--starting-ip-address=192.168.10.2",
		"--keyring-backend=test",
		"--chain-id=chain-test",
		"--additional-sender-account",
		fmt.Sprintf("--slashing-address=%s", slashingAddr),
		fmt.Sprintf("--covenant-pk=%s", covenantPkBip340.MarshalHex()),
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
		babylonNode: newBabylonNode(testDir, startCmd, covenantKeyName, chainID, slashingAddr),
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

func (w *BabylonNodeHandler) GetCovenantKeyName() string {
	return w.babylonNode.covenantKeyName
}

func (w *BabylonNodeHandler) GetSlashingAddress() string {
	return w.babylonNode.slashingAddr
}
