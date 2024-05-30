package e2etest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type WasmdNodeHandler struct {
	wasmdNode *exec.Cmd
	baseDir   string
}

func (w *WasmdNodeHandler) Start() error {
	return w.wasmdNode.Start()
}

func (w *WasmdNodeHandler) Wait() error {
	return w.wasmdNode.Wait()
}

func (w *WasmdNodeHandler) GetNodeDataDir() string {
	return filepath.Join(w.baseDir, "node0", "wasmd")
}

func StartWasmdNode(t *testing.T, chainID, homeDir, babylonContractCode, btcStakingContractCode, instantiatingCfg string) *WasmdNodeHandler {
	wh := NewWasmdNodeHandler(t, chainID, homeDir, babylonContractCode, btcStakingContractCode, instantiatingCfg)
	err := wh.Start()
	require.NoError(t, err)
	// Wait a bit to ensure the node is started
	time.Sleep(10 * time.Second)
	return wh
}

func NewWasmdNodeHandler(t *testing.T, chainID, homeDir, babylonContractCode, btcStakingContractCode, instantiatingCfg string) *WasmdNodeHandler {
	// Prepare directories
	nodeDataDir := filepath.Join(homeDir, "node0", "wasmd")
	err := os.MkdirAll(nodeDataDir, os.ModePerm)
	require.NoError(t, err)

	// Initialize Wasmd testnet files
	initCmd := exec.Command("wasmd", "testnet", "init-files", "--v", "1", fmt.Sprintf("--output-dir=%s", homeDir))
	var stderr bytes.Buffer
	initCmd.Stderr = &stderr
	err = initCmd.Run()
	if err != nil {
		fmt.Printf("wasmd testnet init-files failed: %s \n", stderr.String())
	}
	require.NoError(t, err)

	// Start Wasmd testnet
	startCmd := exec.Command("wasmd", "testnet", "start", fmt.Sprintf("--home=%s", nodeDataDir))
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr

	// Configure Wasmd
	// err = configureWasmd(nodeDataDir)
	// require.NoError(t, err)

	// Upload and instantiate contracts
	err = uploadAndInstantiateContracts(nodeDataDir, chainID, babylonContractCode, btcStakingContractCode, instantiatingCfg)
	require.NoError(t, err)

	return &WasmdNodeHandler{
		wasmdNode: startCmd,
		baseDir:   homeDir,
	}
}

func configureWasmd(nodeDataDir string) error {
	configFilePath := filepath.Join(nodeDataDir, "config", "config.toml")
	appFilePath := filepath.Join(nodeDataDir, "config", "app.toml")

	// Modify configuration files
	err := modifyConfigFile(configFilePath, "tcp://127.0.0.1:26657", "tcp://0.0.0.0:26657")
	if err != nil {
		return err
	}
	err = modifyConfigFile(configFilePath, "tcp://0.0.0.0:26656", "tcp://0.0.0.0:26656")
	if err != nil {
		return err
	}
	err = modifyConfigFile(appFilePath, `"tcp://0.0.0.0:1317"`, `"tcp://0.0.0.0:1318"`)
	return err
}

func modifyConfigFile(filePath, old, new string) error {
	sedCmd := exec.Command("sed", "-i", "", fmt.Sprintf("s#%s#%s#g", old, new), filePath)
	sedCmd.Stderr = os.Stderr
	return sedCmd.Run()
}

func uploadAndInstantiateContracts(nodeDataDir, chainID, babylonContractCode, btcStakingContractCode, instantiatingCfg string) error {
	uploadCmd := exec.Command("wasmd", "tx", "wasm", "store", babylonContractCode, "--keyring-backend", "test", "--from", "user", "--chain-id", chainID, "--gas", "20000000000", "--gas-prices", "0.01ustake", "--home", nodeDataDir, "--yes")
	uploadCmd.Stdout = os.Stdout
	uploadCmd.Stderr = os.Stderr
	if err := uploadCmd.Run(); err != nil {
		return err
	}

	uploadCmd = exec.Command("wasmd", "tx", "wasm", "store", btcStakingContractCode, "--keyring-backend", "test", "--from", "user", "--chain-id", chainID, "--gas", "20000000000", "--gas-prices", "0.01ustake", "--home", nodeDataDir, "--yes")
	uploadCmd.Stdout = os.Stdout
	uploadCmd.Stderr = os.Stderr
	if err := uploadCmd.Run(); err != nil {
		return err
	}

	instantiateCmd := exec.Command("wasmd", "tx", "wasm", "instantiate", "1", instantiatingCfg, "--admin", "$(wasmd --home "+nodeDataDir+" keys show user --keyring-backend test -a)", "--label", "v0.0.1", "--keyring-backend", "test", "--from", "user", "--chain-id", chainID, "--gas", "20000000000", "--gas-prices", "0.001ustake", "--home", nodeDataDir, "--yes")
	instantiateCmd.Stdout = os.Stdout
	instantiateCmd.Stderr = os.Stderr
	return instantiateCmd.Run()
}
