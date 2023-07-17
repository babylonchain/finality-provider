package e2etest

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func baseDir() (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, "zBabylonTest")
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}

type babylonNode struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func newBabylonNode(dataDir string, cmd *exec.Cmd) *babylonNode {
	return &babylonNode{
		dataDir: dataDir,
		cmd:     cmd,
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
	return err
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

func NewBabylonNodeHandler() (*BabylonNodeHandler, error) {
	testDir, err := baseDir()
	if err != nil {
		return nil, err
	}

	initTestnetCmd := exec.Command(
		"babylond",
		"testnet",
		"--v=1",
		fmt.Sprintf("--output-dir=%s", testDir),
		fmt.Sprintf("--home=%s", testDir),
		"--starting-ip-address=192.168.10.2",
		"--keyring-backend=test",
		"--chain-id=chain-1",
		"--additional-sender-account",
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	initTestnetCmd.Stdout = &out
	initTestnetCmd.Stderr = &stderr

	err = initTestnetCmd.Run()

	if err != nil {
		// remove the testDir if this fails
		_ = os.RemoveAll(testDir)
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return nil, err
	}

	nodeDataDir := filepath.Join(testDir, "node0", "babylond")

	startCmd := exec.Command(
		"babylond",
		"start",
		fmt.Sprintf("--home=%s", nodeDataDir),
	)

	return &BabylonNodeHandler{
		babylonNode: newBabylonNode(testDir, startCmd),
	}, nil
}

func (w *BabylonNodeHandler) Start() error {
	if err := w.babylonNode.start(); err != nil {
		return w.babylonNode.cleanup()
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
