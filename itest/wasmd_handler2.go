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
	"time"
)

type wasmdNode struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func newWasmdNode(dataDir string, cmd *exec.Cmd) *wasmdNode {
	return &wasmdNode{
		dataDir: "",
		cmd:     cmd,
	}
}

func (n *wasmdNode) start() error {
	if err := n.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(n.dataDir, fmt.Sprintf("%s.pid", "wasmd")))
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

func (n *wasmdNode) stop() (err error) {
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

func (n *wasmdNode) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile, err)
		}
	}

	//dirs := []string{
	//	n.dataDir,
	//}
	//var err error
	//for _, dir := range dirs {
	//	if err = os.RemoveAll(dir); err != nil {
	//		log.Printf("Cannot remove dir %s: %v", dir, err)
	//	}
	//}
	return nil
}

func (n *wasmdNode) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

type WasmdNodeHandler struct {
	wasmdNode *wasmdNode
}

func NewWasmdNodeHandler(t *testing.T) *WasmdNodeHandler {
	// TODO: utilize testDir, not used anywhere right now cc gurjot
	//testDir, err := baseDir("ZWasmdTest")
	//require.NoError(t, err)
	//defer func() {
	//	if err != nil {
	//		err := os.RemoveAll(testDir)
	//		require.NoError(t, err)
	//	}
	//}()

	//nodeDataDir := filepath.Join(testDir, "wasmd")

	setupScript := filepath.Join("wasmd_scripts", "setup_wasmd.sh")
	startNodeScript := filepath.Join("wasmd_scripts", "start_node.sh")
	accountsScript := filepath.Join("wasmd_scripts", "01-accounts.sh")

	var stderr bytes.Buffer
	initTestnetCmd := exec.Command("/bin/sh", "-c", setupScript)
	initTestnetCmd.Stderr = &stderr

	err := initTestnetCmd.Run()
	if err != nil {
		fmt.Printf("setup wasmd failed: %s \n", stderr.String())
	}
	if err != nil {
		log.Fatalf("Error running setup script: %s\nOutput: %s", setupScript, stderr.String())
	}

	time.Sleep(5 * time.Second) // Adjust the delay as needed

	startCmd := exec.Command("/bin/sh", "-c", startNodeScript)
	//startCmd.Dir = nodeDataDir
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr

	err = startCmd.Start()
	if err != nil {
		log.Fatalf("Error starting wasmd node: %v", err)
	}

	time.Sleep(10 * time.Second) // Adjust the delay as needed

	accountsCmd := exec.Command("/bin/sh", "-c", accountsScript)
	//accountsCmd.Dir = nodeDataDir
	err = accountsCmd.Run()
	if err != nil {
		log.Fatalf("Error running accounts script: %v", err)
	}

	return &WasmdNodeHandler{
		wasmdNode: newWasmdNode("", startCmd),
	}
}

func (w *WasmdNodeHandler) Start() error {
	if err := w.wasmdNode.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.wasmdNode.cleanup()
		return err
	}
	return nil
}

func (w *WasmdNodeHandler) Stop() error {
	if err := w.wasmdNode.shutdown(); err != nil {
		return err
	}

	return nil
}
