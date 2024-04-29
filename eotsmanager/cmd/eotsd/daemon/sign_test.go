package daemon_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	dcli "github.com/babylonchain/finality-provider/eotsmanager/cmd/eotsd/daemon"
	"github.com/babylonchain/finality-provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type FpInfo struct {
	Description stktypes.Description `json:"description"`
	BtcPk       string               `json:"btc_pk"`
	Commision   sdkmath.LegacyDec    `json:"commission"`
}

func FuzzSignAndVerifySchnorrSig(f *testing.F) {
	testutil.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		tempDir := t.TempDir()
		homeDir := filepath.Join(tempDir, "eots-home")
		app := testApp()

		// init config in home folder
		hFlag := fmt.Sprintf("--home=%s", homeDir)
		err := app.Run([]string{"eotsd", "init", hFlag})
		require.NoError(t, err)

		keyName := testutil.GenRandomHexStr(r, 10)
		keyNameFlag := fmt.Sprintf("--key-name=%s", keyName)

		outputKeysAdd := AppRunWithOutput(r, t, app, []string{"eotsd", "keys", "add", hFlag, keyNameFlag})
		keyOutJson := searchInTxt(outputKeysAdd, "for recovery):")

		var keyOut dcli.KeyOutput
		err = json.Unmarshal([]byte(keyOutJson), &keyOut)
		require.NoError(t, err)

		fpInfoPath := filepath.Join(tempDir, "fpInfo.json")
		writeFpInfoToFile(r, t, fpInfoPath, keyOut.PubKeyHex)

		btcPkFlag := fmt.Sprintf("--btc-pk=%s", keyOut.PubKeyHex)
		outputSign := AppRunWithOutput(r, t, app, []string{"eotsd", "sign-schnorr", fpInfoPath, hFlag, btcPkFlag})
		signatureStr := searchInTxt(outputSign, "")

		var dataSigned dcli.DataSigned
		err = json.Unmarshal([]byte(signatureStr), &dataSigned)
		require.NoError(t, err)

		err = app.Run([]string{"eotsd", "verify-schnorr-sig", fpInfoPath, btcPkFlag, fmt.Sprintf("--signature=%s", dataSigned.SchnorrSignatureHex)})
		require.NoError(t, err)
	})
}

func AppRunWithOutput(r *rand.Rand, t *testing.T, app *cli.App, arguments []string) (output string) {
	outPut := filepath.Join(t.TempDir(), fmt.Sprintf("%s-out.txt", testutil.GenRandomHexStr(r, 10)))
	outPutFile, err := os.Create(outPut)
	require.NoError(t, err)
	defer outPutFile.Close()

	// set file to stdout to read.
	os.Stdout = outPutFile

	err = app.Run(arguments)
	require.NoError(t, err)

	return readFromFile(outPutFile)
}

func searchInTxt(text, search string) string {
	idxOfRecovery := strings.Index(text, search)
	jsonKeyOutputOut := text[idxOfRecovery+len(search):]
	return strings.ReplaceAll(jsonKeyOutputOut, "\n", "")
}

func readFromFile(f *os.File) string {
	buf := new(bytes.Buffer)
	f.Seek(0, 0)
	buf.ReadFrom(f)
	return buf.String()
}

func writeFpInfoToFile(r *rand.Rand, t *testing.T, fpInfoPath, btcPk string) {
	desc := testutil.RandomDescription(r)
	fpInfo := FpInfo{
		BtcPk:       btcPk,
		Commision:   sdkmath.LegacyMustNewDecFromStr("0.5"),
		Description: *desc,
	}

	bzFpInfo, err := json.Marshal(fpInfo)
	require.NoError(t, err)

	fpInfoFile, err := os.Create(fpInfoPath)
	require.NoError(t, err)

	_, err = fpInfoFile.Write(bzFpInfo)
	require.NoError(t, err)
	fpInfoFile.Close()
}

func testApp() *cli.App {
	app := cli.NewApp()
	app.Name = "eotsd"
	app.Commands = append(app.Commands, dcli.StartCommand, dcli.InitCommand, dcli.SignSchnorrSig, dcli.VerifySchnorrSig)
	app.Commands = append(app.Commands, dcli.KeysCommands...)
	return app
}
