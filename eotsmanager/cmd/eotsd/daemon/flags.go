package daemon

import "github.com/cosmos/cosmos-sdk/crypto/keyring"

const (
	homeFlag        = "home"
	forceFlag       = "force"
	rpcListenerFlag = "rpc-listener"
	fpPkFlag        = "btc-pk"
	signatureFlag   = "signature"

	// flags for keys
	keyNameFlag        = "key-name"
	passphraseFlag     = "passphrase"
	hdPathFlag         = "hd-path"
	keyringBackendFlag = "keyring-backend"
	recoverFlag        = "recover"

	defaultKeyringBackend = keyring.BackendTest
	defaultHdPath         = ""
	defaultPassphrase     = ""

	// falg for verifier
	verifierFlag = "verifier"
)
