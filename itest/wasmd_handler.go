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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	wasmdRpcPort int = 2990
	wasmdP2pPort int = 2991
	wasmdChainID     = "wasmd-test"
	stake            = "ustake"  // Default staking token
	fee              = "ucosm"   // Default fee token
	moniker          = "node001" // Default moniker
)

type WasmdNodeHandler struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
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

func (w *WasmdNodeHandler) Start() error {
	if err := w.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.cleanup()
		return err
	}
	return nil
}

func (w *WasmdNodeHandler) Stop(t *testing.T) {
	err := w.stop(t)
	if err != nil {
		log.Printf("error stopping wasmd: %v", err)
	}
	require.NoError(t, err)

	err = w.cleanup()
	if err != nil {
		log.Printf("error cleaning up wasmd: %v", err)
	}
	require.NoError(t, err)
}

func (w *WasmdNodeHandler) GetRpcUrl() string {
	return fmt.Sprintf("http://localhost:%d", wasmdRpcPort)
}

func (w *WasmdNodeHandler) GetHomeDir() string {
	return w.dataDir
}

func (w *WasmdNodeHandler) start() error {
	if err := w.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(w.dataDir, fmt.Sprintf("%s.pid", "wasmd")))
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

func (w *WasmdNodeHandler) stop(t *testing.T) (err error) {
	if w.cmd == nil || w.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = w.cmd.Wait()
		if err != nil {
			t.Logf("error waiting for wasmd process to exit: %v", err)
		}
	}()

	if runtime.GOOS == "windows" {
		return w.cmd.Process.Signal(os.Kill)
	}
	t.Logf("sending interrupt signal to wasmd process")
	return w.cmd.Process.Signal(os.Interrupt)
}

func (w *WasmdNodeHandler) cleanup() error {
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

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running command: %v", err)
	}
	return output, nil
}

func wasmdInit(homeDir string) error {
	_, err := runCommand("wasmd", "init", "--home", homeDir, "--chain-id", wasmdChainID, moniker)
	return err
}

func updateGenesisFile(homeDir string) error {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	sedCmd := fmt.Sprintf("sed -i. 's/\"stake\"/\"%s\"/' %s", stake, genesisPath)
	_, err := runCommand("sh", "-c", sedCmd)
	return err
}

func wasmdKeysAdd(homeDir string) error {
	_, err := runCommand("wasmd", "keys", "add", "validator", "--home", homeDir, "--keyring-backend=test")
	return err
}

func addValidatorGenesisAccount(homeDir string) error {
	_, err := runCommand("wasmd", "genesis", "add-genesis-account", "validator", fmt.Sprintf("1000000000000%s,1000000000000%s", stake, fee), "--home", homeDir, "--keyring-backend=test")
	return err
}

func gentxValidator(homeDir string) error {
	_, err := runCommand("wasmd", "genesis", "gentx", "validator", fmt.Sprintf("250000000%s", stake), "--chain-id="+wasmdChainID, "--amount="+fmt.Sprintf("250000000%s", stake), "--home", homeDir, "--keyring-backend=test")
	return err
}

func collectGentxs(homeDir string) error {
	_, err := runCommand("wasmd", "genesis", "collect-gentxs", "--home", homeDir)
	return err
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
		"--from", "validator", "--gas=auto", "--gas-prices=1ustake", "--gas-adjustment=1.3", "-y", "--chain-id", wasmdChainID,
		"--node", w.GetRpcUrl(), "--home", w.GetHomeDir(), "-b", "sync", "-o", "json", "--keyring-backend=test")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("error running wasmd store command: %v", err)
	}

	// TODO: using struct from cometbft/sdk gives unmarshal error, investigate
	var txResp TxResponse
	resp := out.String()
	err = json.Unmarshal([]byte(resp), &txResp)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshalling store wasm response: %v", err)
	}

	time.Sleep(3 * time.Second)
	queryOutput, err := runCommand("wasmd", "--node", w.GetRpcUrl(), "q", "tx", txResp.TxHash, "-o", "json")
	if err != nil {
		return "", "", fmt.Errorf("error querying transaction: %v", err)
	}

	var queryResp TxResponse
	err = json.Unmarshal(queryOutput, &queryResp)
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

func (w *WasmdNodeHandler) GetLatestBlockHeight() (int64, error) {
	out, err := runCommand("wasmd", "status", "--node", w.GetRpcUrl(), "--home", w.GetHomeDir())
	if err != nil {
		return 0, fmt.Errorf("error running wasmd status command: %v", err)
	}

	// TODO: using struct from cometbft/sdk gives unmarshal error, investigate
	var statusResp StatusResponse
	err = json.Unmarshal(out, &statusResp)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling status response: %v", err)
	}

	height, err := strconv.ParseInt(statusResp.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing block height: %v", err)
	}

	return height, nil
}

type ListResponse struct {
	CodeInfos []CodeInfo `json:"code_infos"`
}

type CodeInfo struct {
	CodeID string `json:"code_id"`
}

func (w *WasmdNodeHandler) GetLatestCodeID() (string, error) {
	output, err := runCommand("wasmd", "--node", w.GetRpcUrl(), "q", "wasm", "list-code", "-o", "json")
	if err != nil {
		return "", fmt.Errorf("error running wasmd list-code command: %v", err)
	}

	// TODO: using struct from cometbft/sdk gives unmarshal error, investigate
	var listCodesResp ListResponse
	err = json.Unmarshal(output, &listCodesResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling list-code response: %v", err)
	}

	// Get the latest code ID from the list
	if len(listCodesResp.CodeInfos) == 0 {
		return "", fmt.Errorf("no code info found in list-code response")
	}

	latestCodeID := listCodesResp.CodeInfos[0].CodeID
	return latestCodeID, nil
}
