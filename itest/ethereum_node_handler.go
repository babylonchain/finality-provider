package e2e_utils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type EthereumNodeHandler struct {
	t           *testing.T
	client 			*rpc.Client
}

func NewEthereumNodeHandler(t *testing.T) (*EthereumNodeHandler, error) {
	// Log current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	t.Logf("Current working directory: %s", wd)

	// Start Anvil
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	startScriptPath := filepath.Join(projectRoot, "scripts/start_anvil_devnet.sh")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(startScriptPath, 0755); err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	startCmd := exec.Command("/bin/sh", startScriptPath)
	startCmd.Stdout = &stdout
	startCmd.Stderr = &stderr
	if err := startCmd.Start(); err != nil {
		return nil, err
	}

	// Wait for the command to complete
	startCmd.Wait()

	// Log output and errors
	if stdout.Len() > 0 {
		t.Logf("Anvil stdout: %s", stdout.String())
	}
	if stderr.Len() > 0 {
		t.Logf("Anvil stderr: %s", stderr.String())
	}

	// Connect to local RPC node
	client, err := rpc.DialHTTP("http://localhost:8545")
	if err != nil {
		return nil, err
	}
	
	return &EthereumNodeHandler{
		t:           t,
		client:      client,
	}, nil
}

func (eh *EthereumNodeHandler) Close() {
	if eh.client != nil {
		eh.client.Close()
	}

	// Stop Anvil
	projectRoot, err := findProjectRoot()
	require.NoError(eh.t, err)
	stopScriptPath := filepath.Join(projectRoot, "scripts/stop_anvil_devnet.sh")
	os.Chmod(stopScriptPath, 0755)
	stopCmd := exec.Command("/bin/sh", stopScriptPath)
	stopCmd.Run()
}

// Helper function to find root directory of project
func findProjectRoot() (string, error) {
	_, b, _, _ := runtime.Caller(0)
	dir := filepath.Dir(b)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if parentDir := filepath.Dir(dir); parentDir == dir {
			break
		} else {
			dir = parentDir
		}
	}
	return "", os.ErrNotExist
}