package val

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/store/bbolt"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"
)

func NewCreateValidatorRequest(dbcfg *valcfg.DatabaseConfig) *valrpc.CreateValidatorRequest {
	return &valrpc.CreateValidatorRequest{
		DbOptions: fromDBConfigToOptions(dbcfg),
	}
}

func NewQueryValidatorListRequest(dbcfg *valcfg.DatabaseConfig) *valrpc.QueryValidatorListRequest {
	return &valrpc.QueryValidatorListRequest{
		DbOptions: fromDBConfigToOptions(dbcfg),
	}
}

func fromDBConfigToOptions(dbcfg *valcfg.DatabaseConfig) *valrpc.DbOptions {
	return &valrpc.DbOptions{
		DbType: dbcfg.DbType,
		DbPath: dbcfg.Path,
		DbName: dbcfg.Name,
	}
}

func NewValidator(babylonPk *btcec.PublicKey, btcPk *btcec.PublicKey) *valrpc.Validator {
	return &valrpc.Validator{
		BabylonPk: babylonPk.SerializeCompressed(),
		BtcPk:     btcPk.SerializeCompressed(),
		Status:    valrpc.ValidatorStatus_VALIDATOR_STATUS_CREATED,
	}
}

// CreateValidator generates a validator object and saves it in the database
// the Babylon and BTC private keys are generated randomly and stored in a keyring
// Note: this implementation does not incur rpc connection
func CreateValidator(req *valrpc.CreateValidatorRequest) (*valrpc.CreateValidatorResponse, error) {
	// generate Babylon private key
	bbnPrivKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Babylon private key: %w", err)
	}

	// generate BTC private key
	btcPrivKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate BTC private key: %w", err)
	}

	// build a validator object
	bbnPublicKey := bbnPrivKey.PubKey()
	btcPublicKey := btcPrivKey.PubKey()
	validator := NewValidator(bbnPublicKey, btcPublicKey)

	// TODO secure the private keys in a keyring before storing it in the database

	// open the database
	s, err := OpenStore(req.DbOptions.DbType, req.DbOptions.DbPath, req.DbOptions.DbName)
	defer CloseStore(s)
	if err != nil {
		return nil, fmt.Errorf("failed to open the database: %w", err)
	}

	// save the validator to the database using the Babylon public key as the key
	k := bbnPublicKey.SerializeCompressed()
	v, err := proto.Marshal(validator)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the created validator object: %w", err)
	}

	err = s.Put(k, v)
	if err != nil {
		return nil, fmt.Errorf("failed to save the created validator object: %w", err)
	}

	return &valrpc.CreateValidatorResponse{
		BabylonPk: bbnPublicKey.SerializeCompressed(),
		BtcPk:     btcPublicKey.SerializeCompressed(),
	}, nil
}

// QueryValidatorList returns a list of validators stored in the database
// Note: this implementation does not incur rpc connection
func QueryValidatorList(req *valrpc.QueryValidatorListRequest) (*valrpc.QueryValidatorListResponse, error) {
	// open the database
	s, err := OpenStore(req.DbOptions.DbType, req.DbOptions.DbPath, req.DbOptions.DbName)
	defer CloseStore(s)
	if err != nil {
		return nil, fmt.Errorf("failed to open the database: %w", err)
	}

	valsBytes, err := s.List(nil)
	if err != nil {
		return nil, err
	}

	valsList := make([]*valrpc.Validator, len(valsBytes))
	for i := 0; i < len(valsBytes); i++ {
		val := new(valrpc.Validator)
		err := proto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		valsList[i] = val
	}

	return &valrpc.QueryValidatorListResponse{Validators: valsList}, nil
}

// OpenStore returns a Store instance with the given db type, path and name
// currently, we only support bbolt
func OpenStore(dbType string, path string, name string) (store.Store, error) {
	switch dbType {
	case "bbolt":
		return bbolt.NewBboltStore(path, name)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}

func CloseStore(s store.Store) {
	if err := s.Close(); err != nil {
		panic(err)
	}
}
