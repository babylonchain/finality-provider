package main

import "github.com/cosmos/cosmos-sdk/crypto/keyring"

const (
	homeFlag           = "home"
	forceFlag          = "force"
	allFlag            = "all"
	passphraseFlag     = "passphrase"
	fpPkFlag           = "finality-provider-pk"
	keyNameFlag        = "key-name"
	hdPathFlag         = "hd-path"
	chainIdFlag        = "chain-id"
	keyringBackendFlag = "keyring-backend"
	rpcListenerFlag    = "rpc-listener"

	defaultKeyringBackend = keyring.BackendTest
	defaultHdPath         = ""
	defaultPassphrase     = ""
)
