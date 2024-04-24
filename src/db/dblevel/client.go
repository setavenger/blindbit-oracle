package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"log"
)

// todo change to `var NoEntryErr = errors.new("[no entry found]")`
type NoEntryErr struct{}

func (e NoEntryErr) Error() string {
	return "[no entry found]"
}

var (
	HeadersDB    *leveldb.DB
	HeadersInvDB *leveldb.DB
	FiltersDB    *leveldb.DB
	TweaksDB     *leveldb.DB
	TweakIndexDB *leveldb.DB
	UTXOsDB      *leveldb.DB
)

// OpenDBConnection opens a connection to the through path specified db instance
// if it fails it panics
func OpenDBConnection(path string) *leveldb.DB {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		common.ErrorLogger.Println(err)
		panic(err)
	}
	return db
}

// extractKeyValue will panic because serialisation is critical to data integrity
func extractKeyValue(pair types.Pair) ([]byte, []byte) {
	key, err := pair.SerialiseKey()
	if err != nil {
		common.ErrorLogger.Println(err)
		panic(err)
	}
	value, err := pair.SerialiseData()
	if err != nil {
		common.ErrorLogger.Println(err)
		panic(err)
	}
	return key, value
}

// extractKeyValue will panic because serialisation is critical to data integrity
func extractKey(pair types.Pair) []byte {
	key, err := pair.SerialiseKey()
	if err != nil {
		common.ErrorLogger.Println(err)
		panic(err)
	}
	return key
}

func insertSimple(db *leveldb.DB, pair types.Pair) error {
	key, value := extractKeyValue(pair) // Unpack the key and value
	// Use the key and value as separate arguments for db.Put
	err := db.Put(key, value, nil)
	if err != nil {
		common.ErrorLogger.Println(err)
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
		common.ErrorLogger.Println(err)
		return err
	}
	return err
}

func retrieveByBlockHash(db *leveldb.DB, blockHash string, pair types.Pair) error {
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		log.Println(err)
		return err
	}

	data, err := db.Get(blockHashBytes, nil)
	if err != nil && err.Error() != "leveldb: not found" { // todo this error probably exists as var/type somewhere
		common.ErrorLogger.Println(err)
		return err
	} else if err != nil && err.Error() == "leveldb: not found" { // todo this error probably exists as var/type somewhere
		return NoEntryErr{}
	}
	if len(data) == 0 {
		// todo this should be a different type of error case
		return NoEntryErr{}
	}

	err = pair.DeSerialiseKey(blockHashBytes)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = pair.DeSerialiseData(data)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return nil
}

func retrieveByBlockHeight(db *leveldb.DB, blockHeight uint32, pair types.Pair) error {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, blockHeight)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	data, err := db.Get(buf.Bytes(), nil)
	if err != nil && err.Error() != "leveldb: not found" { // todo this error probably exists as var/type somewhere
		common.ErrorLogger.Println(err)
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
		common.ErrorLogger.Println(err)
		return err
	}

	err = pair.DeSerialiseData(data)
	if err != nil {
		common.ErrorLogger.Println(err)
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
			common.ErrorLogger.Println(err)
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return results, err
}

func retrieveManyByBlockHash(db *leveldb.DB, blockHash string, factory types.PairFactory) ([]types.Pair, error) {
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	iter := db.NewIterator(util.BytesPrefix(blockHashBytes), nil)
	defer iter.Release()
	var results []types.Pair

	for iter.Next() {
		pair := factory()
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return results, err
}

func retrieveManyByBlockHashAndTxid(db *leveldb.DB, blockHash, txid string, factory types.PairFactory) ([]types.Pair, error) {
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	txidBytes, err := hex.DecodeString(txid)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	prefix := append(blockHashBytes, txidBytes...)

	iter := db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	var results []types.Pair

	for iter.Next() {
		pair := factory()
		err = pair.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		err = pair.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		results = append(results, pair)
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
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
		common.ErrorLogger.Println(err)
		return err
	}
	return err
}
