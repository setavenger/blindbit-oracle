package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"errors"
)

func InsertTweakIndex(pair *types.TweakIndex) error {
	err := insertSimple(TweakIndexDB, pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("tweak index inserted")
	return nil
}

func FetchByBlockHashTweakIndex(blockHash string) (*types.TweakIndex, error) {
	var pair types.TweakIndex
	err := retrieveByBlockHash(TweakIndexDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return &pair, nil
}
