package main

import "github.com/cosmos/cosmos-sdk/crypto/keyring"

const (
	homeFlag           = "home"
	forceFlag          = "force"
	passphraseFlag     = "passphrase"
	fpPkFlag           = "btc-pk"
	keyNameFlag        = "key-name"
	hdPathFlag         = "hd-path"
	chainIdFlag        = "chain-id"
	keyringBackendFlag = "keyring-backend"
	rpcListenerFlag    = "rpc-listener"
	recoverFlag        = "recover"

	defaultKeyringBackend = keyring.BackendTest
	defaultHdPath         = ""
	defaultPassphrase     = ""
)
