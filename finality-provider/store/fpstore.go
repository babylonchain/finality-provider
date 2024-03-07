package store

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lightningnetwork/lnd/kvdb"
	pm "google.golang.org/protobuf/proto"

	"github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/proto"
)

var (
	// mapping pk -> proto.FinalityProvider
	finalityProviderBucketName = []byte("finalityProviders")
)

type FinalityProviderStore struct {
	db kvdb.Backend
}

// NewFinalityProviderStore returns a new store backed by db
func NewFinalityProviderStore(cfg *config.DBConfig) (*FinalityProviderStore, error) {
	dbBackend, err := config.GetDbBackend(cfg)
	if err != nil {
		return nil, err
	}

	store := &FinalityProviderStore{dbBackend}
	if err := store.initBuckets(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *FinalityProviderStore) initBuckets() error {
	return kvdb.Batch(s.db, func(tx kvdb.RwTx) error {
		_, err := tx.CreateTopLevelBucket(finalityProviderBucketName)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *FinalityProviderStore) CreateFinalityProvider(
	chainPk *secp256k1.PubKey,
	btcPk *btcec.PublicKey,
	description *stakingtypes.Description,
	commission *sdkmath.LegacyDec,
	keyName, chainId string,
	chainSig, btcSig []byte,
) error {
	desBytes, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("invalid description: %w", err)
	}
	fp := &proto.FinalityProvider{
		ChainPk:     chainPk.Key,
		BtcPk:       schnorr.SerializePubKey(btcPk),
		Description: desBytes,
		Commission:  commission.String(),
		Pop: &proto.ProofOfPossession{
			ChainSig: chainSig,
			BtcSig:   btcSig,
		},
		KeyName: keyName,
		ChainId: chainId,
		Status:  proto.FinalityProviderStatus_CREATED,
	}

	return s.createFinalityProviderInternal(fp)
}

func (s *FinalityProviderStore) createFinalityProviderInternal(
	fp *proto.FinalityProvider,
) error {
	return kvdb.Batch(s.db, func(tx kvdb.RwTx) error {
		fpBucket := tx.ReadWriteBucket(finalityProviderBucketName)
		if fpBucket == nil {
			return ErrCorruptedFinalityProviderDb
		}

		// check btc pk first to avoid duplicates
		if fpBucket.Get(fp.BtcPk) != nil {
			return ErrDuplicateFinalityProvider
		}

		return saveFinalityProvider(fpBucket, fp)
	})
}

func saveFinalityProvider(
	fpBucket walletdb.ReadWriteBucket,
	fp *proto.FinalityProvider,
) error {
	if fp == nil {
		return fmt.Errorf("cannot save nil finality provider")
	}

	marshalled, err := pm.Marshal(fp)
	if err != nil {
		return err
	}

	return fpBucket.Put(fp.BtcPk, marshalled)
}

func (s *FinalityProviderStore) SetFpStatus(btcPk *btcec.PublicKey, status proto.FinalityProviderStatus) error {
	setFpStatus := func(fp *proto.FinalityProvider) error {
		fp.Status = status
		return nil
	}

	return s.setFinalityProviderState(btcPk, setFpStatus)
}

// SetFpLastVotedHeight sets the last voted height to the stored last voted height and last processed height
// only if it is larger than the stored one. This is to ensure the stored state to increase monotonically
func (s *FinalityProviderStore) SetFpLastVotedHeight(btcPk *btcec.PublicKey, lastVotedHeight uint64) error {
	setFpLastVotedHeight := func(fp *proto.FinalityProvider) error {
		if fp.LastVotedHeight < lastVotedHeight {
			fp.LastVotedHeight = lastVotedHeight
		}
		if fp.LastProcessedHeight < lastVotedHeight {
			fp.LastProcessedHeight = lastVotedHeight
		}

		return nil
	}

	return s.setFinalityProviderState(btcPk, setFpLastVotedHeight)
}

// SetFpLastProcessedHeight sets the last processed height to the stored last processed height
// only if it is larger than the stored one. This is to ensure the stored state to increase monotonically
func (s *FinalityProviderStore) SetFpLastProcessedHeight(btcPk *btcec.PublicKey, lastProcessedHeight uint64) error {
	setFpLastProcessedHeight := func(fp *proto.FinalityProvider) error {
		if fp.LastProcessedHeight < lastProcessedHeight {
			fp.LastProcessedHeight = lastProcessedHeight
		}

		return nil
	}

	return s.setFinalityProviderState(btcPk, setFpLastProcessedHeight)
}

func (s *FinalityProviderStore) setFinalityProviderState(
	btcPk *btcec.PublicKey,
	stateTransitionFn func(provider *proto.FinalityProvider) error,
) error {
	pkBytes := schnorr.SerializePubKey(btcPk)
	return kvdb.Batch(s.db, func(tx kvdb.RwTx) error {
		fpBucket := tx.ReadWriteBucket(finalityProviderBucketName)
		if fpBucket == nil {
			return ErrCorruptedFinalityProviderDb
		}

		fpFromDb := fpBucket.Get(pkBytes)
		if fpFromDb == nil {
			return ErrFinalityProviderNotFound
		}

		var storedFp proto.FinalityProvider
		if err := pm.Unmarshal(fpFromDb, &storedFp); err != nil {
			return ErrCorruptedFinalityProviderDb
		}

		if err := stateTransitionFn(&storedFp); err != nil {
			return err
		}

		return saveFinalityProvider(fpBucket, &storedFp)
	})
}

func (s *FinalityProviderStore) GetFinalityProvider(btcPk *btcec.PublicKey) (*StoredFinalityProvider, error) {
	var storedFp *StoredFinalityProvider
	pkBytes := schnorr.SerializePubKey(btcPk)

	err := s.db.View(func(tx kvdb.RTx) error {
		fpBucket := tx.ReadBucket(finalityProviderBucketName)
		if fpBucket == nil {
			return ErrCorruptedFinalityProviderDb
		}

		fpBytes := fpBucket.Get(pkBytes)
		if fpBytes == nil {
			return ErrFinalityProviderNotFound
		}

		var fpProto proto.FinalityProvider
		if err := pm.Unmarshal(fpBytes, &fpProto); err != nil {
			return ErrCorruptedFinalityProviderDb
		}

		fpFromDb, err := protoFpToStoredFinalityProvider(&fpProto)
		if err != nil {
			return err
		}

		storedFp = fpFromDb
		return nil
	}, func() {})

	if err != nil {
		return nil, err
	}

	return storedFp, nil
}

// GetAllStoredFinalityProviders fetches all the stored finality providers from db
// pagination is probably not needed as the expected number of finality providers
// in the store is small
func (s *FinalityProviderStore) GetAllStoredFinalityProviders() ([]*StoredFinalityProvider, error) {
	var storedFps []*StoredFinalityProvider

	err := s.db.View(func(tx kvdb.RTx) error {
		fpBucket := tx.ReadBucket(finalityProviderBucketName)
		if fpBucket == nil {
			return ErrCorruptedFinalityProviderDb
		}

		return fpBucket.ForEach(func(k, v []byte) error {
			var fpProto proto.FinalityProvider
			if err := pm.Unmarshal(v, &fpProto); err != nil {
				return ErrCorruptedFinalityProviderDb
			}

			fpFromDb, err := protoFpToStoredFinalityProvider(&fpProto)
			if err != nil {
				return err
			}
			storedFps = append(storedFps, fpFromDb)

			return nil
		})
	}, func() {})

	if err != nil {
		return nil, err
	}

	return storedFps, nil
}

func (s *FinalityProviderStore) Close() error {
	return s.db.Close()
}
