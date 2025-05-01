package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"
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
	} else if err != nil && errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		common.ErrorLogger.Println(err)
		return nil, err
	}
	// todo this probably does not need to be a pointer
	return &pair, nil
}

// FetchAllTweakIndices returns all types.TweakIndex in the DB
func FetchAllTweakIndices() ([]types.TweakIndex, error) {
	pairs, err := retrieveAll(TweakIndexDB, types.PairFactoryTweakIndex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.TweakIndex, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.TweakIndex); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
