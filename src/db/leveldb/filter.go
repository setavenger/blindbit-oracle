package leveldb

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func SaveFilterTaproot(filter *types.Filter) error {
	err := InsertSimple(filter)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("Taproot Filter inserted")
	return nil
}
