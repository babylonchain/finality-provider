package e2etest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	wasmdRpcPort int = 2990
	wasmdP2pPort int = 2991
	stake            = "ustake"  // Default staking token
	fee              = "ucosm"   // Default fee token
	moniker          = "node001" // Default moniker
)

type WasmdNodeHandler struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
	rpcUrl  string
}

func NewWasmdNodeHandler(t *testing.T) *WasmdNodeHandler {
	testDir, err := baseDir("ZWasmdTest")
	require.NoError(t, err)
	defer func() {
		if err != nil {
			err := os.RemoveAll(testDir)
			require.NoError(t, err)
		}
	}()

	setupWasmd(t, testDir)
	cmd := wasmdStartCmd(t, testDir)
	require.NoError(t, err)

	return &WasmdNodeHandler{
		cmd:     cmd,
		pidFile: "", // empty for now, will be set after start
		dataDir: testDir,
	}
}

func (n *WasmdNodeHandler) Start() error {
	if err := n.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = n.cleanup()
		return err
	}
	return nil
}

func (n *WasmdNodeHandler) Shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

func (n *WasmdNodeHandler) GetRpcUrl() string {
	return fmt.Sprintf("http://localhost:%d", wasmdRpcPort)
}

func (n *WasmdNodeHandler) GetHomeDir() string {
	return n.dataDir
}

// internal function to start the wasmd node
func (n *WasmdNodeHandler) start() error {
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

// internal function to stop the wasmd node
func (n *WasmdNodeHandler) stop() (err error) {
	if n.cmd == nil || n.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = n.cmd.Wait()
	}()

	return n.cmd.Process.Signal(os.Interrupt)
}

// internal function to cleanup the process and data directories
func (n *WasmdNodeHandler) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile, err)
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

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func wasmdInit(homeDir string) error {
	return runCommand("wasmd", "init", "--home", homeDir, "--chain-id", chainID, moniker)
}

func updateGenesisFile(homeDir string) error {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	sedCmd := fmt.Sprintf("sed -i. 's/\"stake\"/\"%s\"/' %s", stake, genesisPath)
	cmd := exec.Command("sh", "-c", sedCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func wasmdKeysAdd(homeDir string) error {
	cmd := exec.Command("wasmd", "keys", "add", "validator", "--home", homeDir, "--keyring-backend=test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func addValidatorGenesisAccount(homeDir string) error {
	cmd := exec.Command("wasmd", "genesis", "add-genesis-account", "validator", fmt.Sprintf("1000000000000%s,1000000000000%s", stake, fee), "--home", homeDir, "--keyring-backend=test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gentxValidator(homeDir string) error {
	cmd := exec.Command("wasmd", "genesis", "gentx", "validator", fmt.Sprintf("250000000%s", stake), "--chain-id="+chainID, "--amount="+fmt.Sprintf("250000000%s", stake), "--home", homeDir, "--keyring-backend=test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func collectGentxs(homeDir string) error {
	return runCommand("wasmd", "genesis", "collect-gentxs", "--home", homeDir)
}

func setupWasmd(t *testing.T, testDir string) {
	err := wasmdInit(testDir)
	require.NoError(t, err)

	err = updateGenesisFile(testDir)
	require.NoError(t, err)

	err = wasmdKeysAdd(testDir)
	require.NoError(t, err)

	err = addValidatorGenesisAccount(testDir)
	require.NoError(t, err)

	err = gentxValidator(testDir)
	require.NoError(t, err)

	err = collectGentxs(testDir)
	require.NoError(t, err)
}

func wasmdStartCmd(t *testing.T, testDir string) *exec.Cmd {
	args := []string{
		"start",
		"--home", testDir,
		"--rpc.laddr", fmt.Sprintf("tcp://0.0.0.0:%d", wasmdRpcPort),
		"--p2p.laddr", fmt.Sprintf("tcp://0.0.0.0:%d", wasmdP2pPort),
		"--log_level=info",
		"--trace",
	}

	f, err := os.Create(filepath.Join(testDir, "wasmd.log"))
	require.NoError(t, err)

	cmd := exec.Command("wasmd", args...)
	cmd.Stdout = f
	cmd.Stderr = f

	return cmd
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

func (w *WasmdNodeHandler) StoreWasmCode(wasmFile string) (string, string, error) {
	cmd := exec.Command("wasmd", "tx", "wasm", "store", wasmFile,
		"--from", "validator", "--gas=auto", "--gas-prices=1ustake", "--gas-adjustment=1.3", "-y", "--chain-id", chainID,
		"--node", w.GetRpcUrl(), "--home", w.GetHomeDir(), "-b", "sync", "-o", "json", "--keyring-backend=test")

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
	queryCmd := exec.Command("wasmd", "--node", w.GetRpcUrl(), "q", "tx", txhash, "-o", "json")

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

type StatusResponse struct {
	SyncInfo struct {
		LatestBlockHeight string `json:"latest_block_height"`
	} `json:"sync_info"`
}

func (w *WasmdNodeHandler) GetLatestBlockHeight() (int, error) {
	cmd := exec.Command("wasmd", "status", "--node", w.GetRpcUrl(), "--home", w.GetHomeDir(), "-o", "json")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("error running wasmd status command: %v", err)
	}

	var statusResp StatusResponse
	err = json.Unmarshal(out.Bytes(), &statusResp)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling status response: %v", err)
	}

	height, err := strconv.Atoi(statusResp.SyncInfo.LatestBlockHeight)
	if err != nil {
		return 0, fmt.Errorf("error converting latest block height to integer: %v", err)
	}

	return height, nil
}
