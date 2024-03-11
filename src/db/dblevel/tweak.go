package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func InsertTweaks(tweaks []types.Tweak) error {
	common.InfoLogger.Println("Inserting tweaks...")
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
	common.InfoLogger.Printf("Inserted %d tweaks", len(tweaks))
	return nil
}

func FetchByBlockHashTweaks(blockHash string) ([]types.Tweak, error) {
	common.InfoLogger.Println("Fetching tweaks")
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
	common.InfoLogger.Printf("Fetched %d tweaks\n", len(result))

	return result, nil
}

func DeleteBatchTweaks(tweaks []types.Tweak) error {
	common.InfoLogger.Println("Deleting Tweaks...")
	if len(tweaks) == 0 {
		common.InfoLogger.Println("no tweaks to delete")
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
	common.InfoLogger.Printf("Deleted %d Tweaks\n", len(tweaks))
	return err
}
