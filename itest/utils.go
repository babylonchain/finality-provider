package e2etest

import (
	"os"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	EventuallyWaitTimeOut = 1 * time.Minute
	EventuallyPollTime    = 500 * time.Millisecond
	FpNamePrefix          = "test-fp-"
	MonikerPrefix         = "moniker-"
	ChainID               = "chain-test"
	Passphrase            = "testpass"
	HdPath                = ""
	BtcNetworkParams      = &chaincfg.SimNetParams
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
