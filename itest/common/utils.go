package e2etest_common

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/babylonchain/babylon/types"
	"github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var (
	EventuallyWaitTimeOut = 2 * time.Minute
	EventuallyPollTime    = 500 * time.Millisecond
	FpNamePrefix          = "test-fp-"
	MonikerPrefix         = "moniker-"
	ChainID               = "chain-test"
	Passphrase            = "testpass"
	HdPath                = ""
	WasmStake             = "ustake"  // Default staking token
	WasmFee               = "ucosm"   // Default fee token
	WasmMoniker           = "node001" // Default moniker
)

func NewDescription(moniker string) *stakingtypes.Description {
	dec := stakingtypes.NewDescription(moniker, "", "", "", "")
	return &dec
}

func BaseDir(pattern string) (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, pattern)
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}

func RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running command: %v", err)
	}
	return output, nil
}

func GenerateCovenantCommittee(numCovenants int, t *testing.T) ([]*btcec.PrivateKey, []*types.BIP340PubKey) {
	var (
		covenantPrivKeys []*btcec.PrivateKey
		covenantPubKeys  []*types.BIP340PubKey
	)

	for i := 0; i < numCovenants; i++ {
		privKey, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		covenantPrivKeys = append(covenantPrivKeys, privKey)
		pubKey := types.NewBIP340PubKeyFromBTCPK(privKey.PubKey())
		covenantPubKeys = append(covenantPubKeys, pubKey)
	}

	return covenantPrivKeys, covenantPubKeys
}

func DefaultFpConfig(keyringDir, homeDir string) *config.Config {
	cfg := config.DefaultConfigWithHome(homeDir)

	cfg.BitcoinNetwork = "simnet"
	cfg.BTCNetParams = chaincfg.SimNetParams

	cfg.PollerConfig.AutoChainScanningMode = false
	// babylon configs for sending transactions
	cfg.BabylonConfig.KeyDirectory = keyringDir
	// need to use this one to send otherwise we will have account sequence mismatch
	// errors
	cfg.BabylonConfig.Key = "test-spending-key"
	// Big adjustment to make sure we have enough gas in our transactions
	cfg.BabylonConfig.GasAdjustment = 20

	return &cfg
}
