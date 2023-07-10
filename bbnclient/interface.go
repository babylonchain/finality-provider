package babylonclient

import (
	"github.com/btcsuite/btcd/btcec/v2"
)

type ValidatorParams struct {
	// Bitcoin public key of the current jury
	JuryPk btcec.PublicKey
}

type BabylonClient interface {
	Params() (*ValidatorParams, error)
}

type MockBabylonClient struct {
	ClientParams *ValidatorParams
}

var _ BabylonClient = (*MockBabylonClient)(nil)

func (m *MockBabylonClient) Params() (*ValidatorParams, error) {
	return m.ClientParams, nil
}

func GetMockClient() *MockBabylonClient {
	juryPk, err := btcec.NewPrivateKey()

	if err != nil {
		panic(err)
	}

	return &MockBabylonClient{
		ClientParams: &ValidatorParams{
			JuryPk: *juryPk.PubKey(),
		},
	}
}
