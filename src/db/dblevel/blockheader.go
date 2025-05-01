package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"
)

func InsertBlockHeader(pair types.BlockHeader) error {
	err := insertSimple(HeadersDB, &pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("block_header inserted")
	return nil
}

func FetchByBlockHashBlockHeader(blockHash string) (*types.BlockHeader, error) {
	var pair types.BlockHeader
	err := retrieveByBlockHash(HeadersDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		//common.ErrorLogger.Println(err) don't print case is ignored above anyways
		return nil, err
	}
	return &pair, nil
}
