package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func InsertFilter(pair types.Filter) error {
	err := insertSimple(FiltersDB, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("Taproot Filter inserted")
	return nil
}

func FetchByBlockHashFilter(blockHash string) (types.Filter, error) {
	var pair types.Filter
	err := retrieveByBlockHash(FiltersDB, blockHash, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return types.Filter{}, err
	}
	return pair, nil
}
