package valcfg

import "github.com/btcsuite/btcd/chaincfg"

var (
	defaultBitcoinNetwork  = "simnet"
	defaultJuryKeyName     = "jury-key"
	defaultActiveNetParams = chaincfg.SimNetParams
)

type JuryConfig struct {
	JuryKeyName    string `long:"jurykeyname" description:"The key name of the Jury if the program is running in Jury mode"`
	BitcoinNetwork string `long:"bitcoinnetwork" description:"Bitcoin network to run on" choice:"regtest" choice:"testnet" choice:"simnet" choice:"signet"`

	ActiveNetParams chaincfg.Params
}

func DefaultJuryConfig() JuryConfig {
	return JuryConfig{
		JuryKeyName:     defaultJuryKeyName,
		BitcoinNetwork:  defaultBitcoinNetwork,
		ActiveNetParams: defaultActiveNetParams,
	}
}
