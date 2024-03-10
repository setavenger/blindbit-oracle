package leveldb

import (
	"SilentPaymentAppBackend/src/common"
	"github.com/syndtr/goleveldb/leveldb"
)

const dbPath = "./data"

const (
	dbPathHeaders = dbPath + "/headers"
	dbPathFilter  = dbPath + "/filters"
	dbPathTweaks  = dbPath + "/tweaks"
	dbPathUTXOs   = dbPath + "/utxos"
)

type KeyValuePair interface {
	GetKey() ([]byte, error) // in case it fails we can abort
	Serialise() ([]byte, error)
	DeSerialise([]byte) error // should return an instance of the struct itself
}

func InsertSimple(pair KeyValuePair) error {
	db, err := leveldb.OpenFile(dbPathFilter, nil)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	defer func(db *leveldb.DB) {
		err = db.Close()
		if err != nil {
			common.WarningLogger.Println(err)
		}
	}(db)

	key, err := pair.GetKey()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	value, err := pair.Serialise()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	err = db.Put(key, value, nil)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return nil
}
