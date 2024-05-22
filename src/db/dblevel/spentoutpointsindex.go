package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"errors"
)

func InsertSpentOutpointsIndex(pair *types.SpentOutpointsIndex) error {
	err := insertSimple(SpentOutpointsIndexDB, pair)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("tweak index inserted")
	return nil
}

func FetchByBlockHashSpentOutpointIndex(blockHash string) (*types.SpentOutpointsIndex, error) {
	var pair types.SpentOutpointsIndex
	err := retrieveByBlockHash(SpentOutpointsIndexDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return &pair, nil
}

// FetchAllTweakIndices returns all types.TweakIndex in the DB
func FetchAllSpenOutpointsIndices() ([]types.SpentOutpointsIndex, error) {
	pairs, err := retrieveAll(SpentOutpointsIndexDB, types.PairFactorySpentOutpointsIndex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.SpentOutpointsIndex, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.SpentOutpointsIndex); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
