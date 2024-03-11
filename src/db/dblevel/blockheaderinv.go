package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func InsertBlockHeaderInv(pair types.BlockHeaderInv) error {
	err := insertSimple(HeadersInvDB, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("block_header inserted")
	return nil
}

func FetchByBlockHeightBlockHeaderInv(height uint32) (types.BlockHeaderInv, error) {
	var pair types.BlockHeaderInv
	err := retrieveByBlockHeight(HeadersInvDB, height, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return types.BlockHeaderInv{}, err
	}
	return pair, nil
}

func FetchHighestBlockHeaderInv() (*types.BlockHeaderInv, error) {
	// Assume the height is stored as a big-endian uint32 key
	// Create an iterator that iterates in reverse order
	iter := HeadersInvDB.NewIterator(nil, nil)
	defer iter.Release()
	var result types.BlockHeaderInv

	if iter.Last() {
		// Deserialize the key to get the height
		err := result.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		err = result.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
	}

	err := iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return &result, err
}
