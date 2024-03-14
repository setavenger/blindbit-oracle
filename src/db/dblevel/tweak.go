package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func InsertBatchTweaks(tweaks []types.Tweak) error {
	common.DebugLogger.Println("Inserting tweaks...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(tweaks))

	// Convert each Tweak to a Pair and assign it to the new slice
	for i, pair := range tweaks {
		pairs[i] = &pair
	}

	err := insertBatch(TweaksDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Inserted %d tweaks", len(tweaks))
	return nil
}

func FetchByBlockHashTweaks(blockHash string) ([]types.Tweak, error) {
	common.DebugLogger.Println("Fetching tweaks")
	pairs, err := retrieveManyByBlockHash(TweaksDB, blockHash, types.PairFactoryTweak)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, NoEntryErr{}
	}

	result := make([]types.Tweak, len(pairs))
	// Convert each Tweak to a Pair and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.Tweak); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	common.DebugLogger.Printf("Fetched %d tweaks\n", len(result))

	return result, nil
}

func DeleteBatchTweaks(tweaks []types.Tweak) error {
	common.DebugLogger.Println("Deleting Tweaks...")
	if len(tweaks) == 0 {
		common.DebugLogger.Println("no tweaks to delete")
		return nil
	}
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(tweaks))

	// Convert each Tweak to a Pair and assign it to the new slice
	for i, pair := range tweaks {
		pairs[i] = &pair
	}
	err := deleteBatch(TweaksDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Deleted %d Tweaks\n", len(tweaks))
	return err
}

// FetchAllTweaks returns all types.Tweak in the DB
func FetchAllTweaks() ([]types.Tweak, error) {
	pairs, err := retrieveAll(TweaksDB, types.PairFactoryTweak)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.Tweak, len(pairs))
	// Convert each Pair to a Tweak and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.Tweak); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
