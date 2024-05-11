package keymanager

type KeyManager interface {
	// CreateKeyPair generates a key pair, stores it in the key directory,
	// encrypts it with the given 'passphrase', and then returns the address.
	CreateKeyPair(passphrase string) (string, error)
	// GetPrivkey retrun the private key of 'addr',
	// decrypting it with the given 'passphrase'.
	GetPrivkey(addr, passphrase string) (string, error)
}
