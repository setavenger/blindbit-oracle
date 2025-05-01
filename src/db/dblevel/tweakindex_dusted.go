package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"
)

func InsertTweakIndexDust(pair *types.TweakIndexDust) error {
	err := insertSimple(TweakIndexDustDB, pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("tweak index with dust filter inserted")
	return nil
}

func FetchByBlockHashTweakIndexDust(blockHash string) (*types.TweakIndexDust, error) {
	var pair types.TweakIndexDust
	err := retrieveByBlockHash(TweakIndexDustDB, blockHash, &pair)
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

// FetchAllTweakIndicesDust returns all types.TweakIndexDust in the DB
func FetchAllTweakIndicesDust() ([]types.TweakIndexDust, error) {
	pairs, err := retrieveAll(TweakIndexDustDB, types.PairFactoryTweakIndexDust)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.TweakIndexDust, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.TweakIndexDust); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
