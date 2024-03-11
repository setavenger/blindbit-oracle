package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"errors"
)

func InsertBlockHeader(pair types.BlockHeader) error {
	err := insertSimple(HeadersDB, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("block_header inserted")
	return nil
}

func FetchByBlockHashBlockHeader(blockHash string) (*types.BlockHeader, error) {
	var pair types.BlockHeader
	err := retrieveByBlockHash(HeadersDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) {
		return nil, err
	}
	return &pair, nil
}
