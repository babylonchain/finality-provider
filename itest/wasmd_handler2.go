package e2etest

import (
	"bytes"
	"encoding/json"
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

type TxResponse struct {
	TxHash string `json:"txhash"`
	Events []struct {
		Type       string `json:"type"`
		Attributes []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"attributes"`
	} `json:"events"`
}

func (w *WasmdNodeHandler) StoreWasmCode() (string, string, error) {
	dir := "/Users/gusin/Github/finality-provider/itest/wasmd_contracts"
	cmd := exec.Command("wasmd", "tx", "wasm", "store", fmt.Sprintf("%s/babylon_contract.wasm", dir),
		"--from", "validator", "--gas=auto", "--gas-prices=1ustake", "--gas-adjustment=1.3", "-y", "--chain-id=testing",
		"--node=http://localhost:26657", "-b", "sync", "-o", "json", "--keyring-backend=test")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("error running wasmd store command: %v", err)
	}

	var txResp TxResponse
	resp := out.String()
	err = json.Unmarshal([]byte(resp), &txResp)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshalling store wasm response: %v", err)
	}

	// Wait for a few seconds to ensure the transaction is processed
	time.Sleep(6 * time.Second)

	txhash := txResp.TxHash
	queryCmd := exec.Command("wasmd", "q", "tx", txhash, "-o", "json")

	var queryOut bytes.Buffer
	queryCmd.Stdout = &queryOut
	queryCmd.Stderr = os.Stderr

	err = queryCmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("error querying transaction: %v", err)
	}

	var queryResp TxResponse
	err = json.Unmarshal(queryOut.Bytes(), &queryResp)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshalling query response: %v", err)
	}

	var codeID, codeHash string
	for _, event := range queryResp.Events {
		if event.Type == "store_code" {
			for _, attr := range event.Attributes {
				if attr.Key == "code_id" {
					codeID = attr.Value
				} else if attr.Key == "code_checksum" {
					codeHash = attr.Value
				}
			}
		}
	}

	if codeID == "" || codeHash == "" {
		return "", "", fmt.Errorf("code ID or code checksum not found in transaction events")
	}

	return codeID, codeHash, nil
}
