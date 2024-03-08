package store

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightningnetwork/lnd/kvdb"

	eotscfg "github.com/babylonchain/finality-provider/eotsmanager/config"
)

var (
	eotsBucketName = []byte("fpKeyNames")
)

type EOTSStore struct {
	db kvdb.Backend
}

func NewEOTSStore(cfg *eotscfg.DBConfig) (*EOTSStore, error) {
	dbBackend, err := cfg.GetDbBackend()
	if err != nil {
		return nil, err
	}
	s := &EOTSStore{dbBackend}
	if err := s.initBuckets(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *EOTSStore) initBuckets() error {
	return kvdb.Batch(s.db, func(tx kvdb.RwTx) error {
		_, err := tx.CreateTopLevelBucket(eotsBucketName)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *EOTSStore) Close() error {
	if err := s.db.Close(); err != nil {
		return err
	}

	return nil
}

func (s *EOTSStore) AddEOTSKeyName(
	btcPk *btcec.PublicKey,
	keyName string,
) error {
	pkBytes := schnorr.SerializePubKey(btcPk)

	return kvdb.Batch(s.db, func(tx kvdb.RwTx) error {
		eotsBucket := tx.ReadWriteBucket(eotsBucketName)
		if eotsBucket == nil {
			return ErrCorruptedEOTSDb
		}

		// check btc pk first to avoid duplicates
		if eotsBucket.Get(pkBytes) != nil {
			return ErrDuplicateEOTSKeyName
		}

		return saveEOTSKeyName(eotsBucket, pkBytes, keyName)
	})
}

func saveEOTSKeyName(
	eotsBucket walletdb.ReadWriteBucket,
	btcPk []byte,
	keyName string,
) error {
	if keyName == "" {
		return fmt.Errorf("cannot save empty key name")
	}

	return eotsBucket.Put(btcPk, []byte(keyName))
}

func (s *EOTSStore) GetEOTSKeyName(pk []byte) (string, error) {
	var keyName string
	err := s.db.View(func(tx kvdb.RTx) error {
		eotsBucket := tx.ReadBucket(eotsBucketName)
		if eotsBucket == nil {
			return ErrCorruptedEOTSDb
		}

		keyNameBytes := eotsBucket.Get(pk)
		if keyNameBytes == nil {
			return ErrEOTSKeyNameNotFound
		}

		keyName = string(keyNameBytes)
		return nil
	}, func() {})

	if err != nil {
		return "", err
	}

	return keyName, nil
}
