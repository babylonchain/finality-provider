package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/log"
	"github.com/urfave/cli"
)

type DataSigned struct {
	KeyName             string `json:"key_name"`
	PubKeyHex           string `json:"pub_key_hex"`
	SignedDataHashHex   string `json:"signed_data_hash_hex"`
	SchnorrSignatureHex string `json:"schnorr_signature_hex"`
}

var SignSchnorrSig = cli.Command{
	Name:      "sign-schnorr",
	Usage:     "Signs a Schnorr signature over arbitrary data with the EOTS private key.",
	UsageText: "sign-schnorr [file-path] --key-name [my-key-name]",
	Description: `Read the file received as argument, hash it with
	sha256 and sign based on the Schnorr key associated with the key-name flag`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  homeFlag,
			Usage: "Path to the keyring directory",
			Value: config.DefaultEOTSDir,
		},
		cli.StringFlag{
			Name:     keyNameFlag,
			Required: true,
		},
		cli.StringFlag{
			Name:  passphraseFlag,
			Usage: "The passphrase used to decrypt the keyring",
			Value: defaultPassphrase,
		},
	},
	Action: SignSchnorr,
}

func SignSchnorr(ctx *cli.Context) error {
	keyName := ctx.String(keyNameFlag)
	passphrase := ctx.String(passphraseFlag)

	args := ctx.Args()
	inputFilePath := args.First()
	if len(inputFilePath) == 0 {
		return errors.New("invalid argument, please provide a valid file path as input argument")
	}

	homePath, err := getHomeFlag(ctx)
	if err != nil {
		return fmt.Errorf("failed to load home flag: %w", err)
	}

	cfg, err := config.LoadConfig(homePath)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", homePath, err)
	}

	logger, err := log.NewRootLoggerWithFile(config.LogFile(homePath), cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to load the logger")
	}

	dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
	if err != nil {
		return fmt.Errorf("failed to create db backend: %w", err)
	}

	eotsManager, err := eotsmanager.NewLocalEOTSManager(homePath, cfg, dbBackend, logger)
	if err != nil {
		return fmt.Errorf("failed to create EOTS manager: %w", err)
	}

	h := sha256.New()

	f, err := os.Open(inputFilePath)
	if err != nil {
		return fmt.Errorf("failed to open the file %s: %w", inputFilePath, err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	hashOfMsgToSign := h.Sum(nil)
	signature, pubKey, err := eotsManager.SignSchnorrSigFromKeyname(keyName, hashOfMsgToSign, passphrase)
	if err != nil {
		return fmt.Errorf("failed to sign msg: %w", err)
	}

	printRespJSON(DataSigned{
		KeyName:             keyName,
		PubKeyHex:           pubKey.MarshalHex(),
		SignedDataHashHex:   hex.EncodeToString(hashOfMsgToSign),
		SchnorrSignatureHex: hex.EncodeToString(signature.Serialize()),
	})

	return nil
}

func printRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}
