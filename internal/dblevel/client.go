package dblevel

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// todo change to `var NoEntryErr = errors.new("[no entry found]")`
type NoEntryErr struct{}

func (e NoEntryErr) Error() string {
	return "[no entry found]"
}

var (
	HeadersDB              *leveldb.DB
	HeadersInvDB           *leveldb.DB
	NewUTXOsFiltersDB      *leveldb.DB
	TweaksDB               *leveldb.DB
	TweakIndexDB           *leveldb.DB
	TweakIndexDustDB       *leveldb.DB
	UTXOsDB                *leveldb.DB
	SpentOutpointsIndexDB  *leveldb.DB
	SpentOutpointsFilterDB *leveldb.DB
)

// OpenDBConnection opens a connection to the through path specified db instance
// if it fails it panics
func OpenDBConnection(path string) *leveldb.DB {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		logging.L.Err(err).Msg("error opening db connection")
		panic(err)
	}
	return db
}

// extractKeyValue will panic because serialisation is critical to data integrity
func extractKeyValue(pair types.Pair) ([]byte, []byte) {
	key, err := pair.SerialiseKey()
	if err != nil {
		logging.L.Err(err).Msg("error serialising key")
		panic(err)
	}
	value, err := pair.SerialiseData()
	if err != nil {
		logging.L.Err(err).Msg("error serialising data")
		panic(err)
	}
	return key, value
}

// extractKeyValue will panic because serialisation is critical to data integrity
func extractKey(pair types.Pair) []byte {
	key, err := pair.SerialiseKey()
	if err != nil {
		logging.L.Err(err).Msg("error serialising key")
		panic(err)
	}
	return key
}

func insertSimple(db *leveldb.DB, pair types.Pair) error {
	key, value := extractKeyValue(pair) // Unpack the key and value
	// Use the key and value as separate arguments for db.Put
	err := db.Put(key, value, nil)
	if err != nil {
		logging.L.Err(err).Msg("error inserting simple")
		return err
	}

	return nil
}

func insertBatch(db *leveldb.DB, pairs []types.Pair) error {
	batch := new(leveldb.Batch)
	for _, pair := range pairs {
		key, value := extractKeyValue(pair) // Unpack the key and value
		batch.Put(key, value)
	}

	err := db.Write(batch, nil)
	if err != nil {
		logging.L.Err(err).Msg("error inserting batch")
		return err
	}
	return err
}

func retrieveByBlockHash(db *leveldb.DB, blockHash [32]byte, pair types.Pair) error {
	data, err := db.Get(blockHash[:], nil)
	if err != nil && !errors.Is(err, leveldb.ErrNotFound) { // todo this error probably exists as var/type somewhere
		logging.L.Err(err).Msg("error getting block hash")
		return err
	} else if err != nil && errors.Is(err, leveldb.ErrNotFound) { // todo this error probably exists as var/type somewhere
		// todo we don't need separate patterns if just return the errors anyways? or maybe just to avoid unnecessary logging
		return NoEntryErr{}
	}
	if len(data) == 0 {
		// todo this should be a different type of error case
		return NoEntryErr{}
	}

	err = pair.DeSerialiseKey(blockHash[:])
	if err != nil {
		logging.L.Err(err).Msg("error deserialising key")
		return err
	}

	err = pair.DeSerialiseData(data)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising data")
		return err
	}

	return nil
}

func retrieveByBlockHeight(db *leveldb.DB, blockHeight uint32, pair types.Pair) error {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, blockHeight)
	if err != nil {
		logging.L.Err(err).Msg("error writing block height")
		return err
	}
	data, err := db.Get(buf.Bytes(), nil)
	if err != nil && err.Error() != "leveldb: not found" { // todo this error probably exists as var/type somewhere
		logging.L.Err(err).Msg("error getting block height")
		return err
	} else if err != nil && err.Error() == "leveldb: not found" { // todo this error probably exists as var/type somewhere
		return NoEntryErr{}
	}

	if len(data) == 0 {
		// todo this should be a different type of error case
		return NoEntryErr{}
	}

	err = pair.DeSerialiseKey(buf.Bytes())
	if err != nil {
		logging.L.Err(err).Msg("error deserialising key")
		return err
	}

	err = pair.DeSerialiseData(data)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising data")
		return err
	}

	return nil
}

func retrieveAll(db *leveldb.DB, factory types.PairFactory) ([]types.Pair, error) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	var results []types.Pair

	var err error
	for iter.Next() {
		pair := factory()
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising key")
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising data")
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		logging.L.Err(err).Msg("error iterating over db")
		return nil, err
	}
	return results, err
}

func retrieveManyByBlockHash(db *leveldb.DB, blockHash [32]byte, factory types.PairFactory) ([]types.Pair, error) {
	blockHashBytes := blockHash[:]
	iter := db.NewIterator(util.BytesPrefix(blockHashBytes), nil)
	defer iter.Release()

	var err error
	var results []types.Pair

	for iter.Next() {
		pair := factory()
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising key")
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising data")
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		logging.L.Err(err).Msg("error iterating over db")
		return nil, err
	}
	return results, err
}

func retrieveManyByBlockHashAndTxid(db *leveldb.DB, blockHash, txid [32]byte, factory types.PairFactory) ([]types.Pair, error) {
	var prefix [64]byte
	copy(prefix[:32], blockHash[:])
	copy(prefix[32:], txid[:])

	iter := db.NewIterator(util.BytesPrefix(prefix[:]), nil)
	defer iter.Release()

	var err error
	var results []types.Pair

	for iter.Next() {
		pair := factory()
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising key")
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising data")
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		logging.L.Err(err).Msg("error iterating over db")
		return nil, err
	}
	if results == nil {
		return nil, NoEntryErr{}
	}
	return results, err
}

func deleteBatch(db *leveldb.DB, pairs []types.Pair) error {
	batch := new(leveldb.Batch)
	for _, pair := range pairs {
		key := extractKey(pair)
		batch.Delete(key)
	}

	err := db.Write(batch, nil)
	if err != nil {
		logging.L.Err(err).Msg("error deleting batch")
		return err
	}
	return err
}
