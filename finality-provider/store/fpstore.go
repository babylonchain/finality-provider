package store

import (
	"fmt"
	"math"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/store"
)

const (
	fpPrefix = "finality-provider"
)

func NewStoreFinalityProvider(babylonPk *secp256k1.PubKey, btcPk *types.BIP340PubKey, keyName, chainID string, pop *bstypes.ProofOfPossession, des []byte, com *sdkmath.LegacyDec) *proto.StoreFinalityProvider {
	return &proto.StoreFinalityProvider{
		KeyName:   keyName,
		BabylonPk: babylonPk.Bytes(),
		BtcPk:     btcPk.MustMarshal(),
		Pop: &proto.ProofOfPossession{
			BabylonSig: pop.BabylonSig,
			BtcSig:     pop.BtcSig,
		},
		ChainId:     chainID,
		Status:      proto.FinalityProviderStatus_CREATED,
		Description: des,
		Commission:  com.String(),
	}
}

type FinalityProviderStore struct {
	s store.Store
}

func NewFinalityProviderStore(dbPath string, dbName string, dbBackend string) (*FinalityProviderStore, error) {
	s, err := openStore(dbPath, dbName, dbBackend)
	if err != nil {
		return nil, err
	}

	return &FinalityProviderStore{s: s}, nil
}

func getFinalityProviderKey(pk []byte) []byte {
	return append([]byte(fpPrefix), pk...)
}

func (vs *FinalityProviderStore) getFinalityProviderListKey() []byte {
	return []byte(fpPrefix)
}

func (vs *FinalityProviderStore) SaveFinalityProvider(fp *proto.StoreFinalityProvider) error {
	k := getFinalityProviderKey(fp.BtcPk)
	v, err := gproto.Marshal(fp)
	if err != nil {
		return fmt.Errorf("failed to marshal the created finality-provider object: %w", err)
	}

	if err := vs.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the created finality-provider object: %w", err)
	}

	return nil
}

func (vs *FinalityProviderStore) UpdateFinalityProvider(fp *proto.StoreFinalityProvider) error {
	k := getFinalityProviderKey(fp.BtcPk)
	exists, err := vs.s.Exists(k)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the finality-provider does not exist")
	}

	v, err := gproto.Marshal(fp)
	if err != nil {
		return err
	}

	if err := vs.s.Put(k, v); err != nil {
		return err
	}

	return nil
}

func (vs *FinalityProviderStore) SetFinalityProviderStatus(fp *proto.StoreFinalityProvider, status proto.FinalityProviderStatus) error {
	fp.Status = status
	return vs.UpdateFinalityProvider(fp)
}

func (vs *FinalityProviderStore) GetStoreFinalityProvider(pk []byte) (*proto.StoreFinalityProvider, error) {
	k := getFinalityProviderKey(pk)
	fpBytes, err := vs.s.Get(k)
	if err != nil {
		return nil, err
	}

	fp := new(proto.StoreFinalityProvider)
	err = gproto.Unmarshal(fpBytes, fp)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal finality-provider object: %w", err))
	}

	return fp, nil
}

func (vs *FinalityProviderStore) ListFinalityProviders() ([]*proto.StoreFinalityProvider, error) {
	k := vs.getFinalityProviderListKey()
	fpsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	fpsList := make([]*proto.StoreFinalityProvider, len(fpsBytes))
	for i := 0; i < len(fpsBytes); i++ {
		fp := new(proto.StoreFinalityProvider)
		err := gproto.Unmarshal(fpsBytes[i].Value, fp)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal finality-provider from the database: %w", err))
		}
		fpsList[i] = fp
	}

	return fpsList, nil
}

// ListRegisteredFinalityProviders returns a list of finality providers whose status is more than CREATED
// but less than SLASHED
func (vs *FinalityProviderStore) ListRegisteredFinalityProviders() ([]*proto.StoreFinalityProvider, error) {
	k := vs.getFinalityProviderListKey()
	fpsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	fpsList := make([]*proto.StoreFinalityProvider, 0)
	for i := 0; i < len(fpsBytes); i++ {
		fp := new(proto.StoreFinalityProvider)
		err := gproto.Unmarshal(fpsBytes[i].Value, fp)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal finality-provider from the database: %w", err))
		}
		if fp.Status > proto.FinalityProviderStatus_CREATED && fp.Status < proto.FinalityProviderStatus_SLASHED {
			fpsList = append(fpsList, fp)
		}
	}

	return fpsList, nil
}

func (vs *FinalityProviderStore) GetEarliestActiveFinalityProviderVotedHeight() (uint64, error) {
	registeredFps, err := vs.ListRegisteredFinalityProviders()
	if err != nil {
		return 0, err
	}

	if len(registeredFps) == 0 {
		return 0, nil
	}

	earliestHeight := uint64(math.MaxUint64)
	activeFpsCnt := 0
	for _, fp := range registeredFps {
		// Note there might be a delay between the finality-provider being active on Babylon
		// and this program capturing that. However, given that we only care
		// about the `LastVotedHeight` of the finality-provider, other parts of the program
		// ensure that when this value is set, the finality-provider is stored as ACTIVE.
		// TODO: Another option would be to query here for the
		// active status of each finality-provider although this might prove inefficient.
		if fp.Status != proto.FinalityProviderStatus_ACTIVE {
			continue
		}
		activeFpsCnt += 1
		if earliestHeight > fp.LastVotedHeight {
			earliestHeight = fp.LastVotedHeight
		}
	}
	// If there are no active finality providers, return 0
	if activeFpsCnt == 0 {
		return 0, nil
	}
	return earliestHeight, nil
}

func (vs *FinalityProviderStore) Close() error {
	if err := vs.s.Close(); err != nil {
		return err
	}

	return nil
}

// openStore returns a Store instance with the given db type, path and name
// currently, we only support bbolt
func openStore(dbPath string, dbName string, dbBackend string) (store.Store, error) {
	switch dbBackend {
	case "bbolt":
		return store.NewBboltStore(dbPath, dbName)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}
