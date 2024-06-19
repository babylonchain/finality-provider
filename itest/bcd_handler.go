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
	bcdRpcPort int = 2990
	bcdP2pPort int = 2991
	bcdChainID     = "bcd-test"
)

type BcdNodeHandler struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func NewBcdNodeHandler(t *testing.T) *BcdNodeHandler {
	testDir, err := baseDir("ZBcdTest")
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
	_, err := runCommand("bcd", "init", "--home", homeDir, "--chain-id", bcdChainID, moniker)
	return err
}

func bcdUpdateGenesisFile(homeDir string) error {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	sedCmd := fmt.Sprintf("sed -i. 's/\"stake\"/\"%s\"/' %s", stake, genesisPath)
	_, err := runCommand("sh", "-c", sedCmd)
	return err
}

func bcdKeysAdd(homeDir string) error {
	_, err := runCommand("bcd", "keys", "add", "validator", "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdAddValidatorGenesisAccount(homeDir string) error {
	_, err := runCommand("bcd", "genesis", "add-genesis-account", "validator", fmt.Sprintf("1000000000000%s,1000000000000%s", stake, fee), "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdGentxValidator(homeDir string) error {
	_, err := runCommand("bcd", "genesis", "gentx", "validator", fmt.Sprintf("250000000%s", stake), "--chain-id="+bcdChainID, "--amount="+fmt.Sprintf("250000000%s", stake), "--home", homeDir, "--keyring-backend=test")
	return err
}

func bcdCollectGentxs(homeDir string) error {
	_, err := runCommand("bcd", "genesis", "collect-gentxs", "--home", homeDir)
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

func (w *BcdNodeHandler) StoreWasmCode(wasmFile string) (string, string, error) {
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

func (w *BcdNodeHandler) GetLatestBlockHeight() (int64, error) {
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

func (w *BcdNodeHandler) GetLatestCodeID() (string, error) {
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
