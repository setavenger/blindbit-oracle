package dblevel

import (
	"encoding/hex"
	"errors"
	"math"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"

	"github.com/syndtr/goleveldb/leveldb/util"
)

// InsertBatchTweaks index implements cut through and dust
func InsertBatchTweaks(tweaks []types.Tweak) error {
	// common.DebugLogger.Println("Inserting tweaks...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(tweaks))

	// Convert each Tweak to a Pair and assign it to the new slice
	for i, pair := range tweaks {
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = &pairCopy
	}

	err := insertBatch(TweaksDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Inserted %d tweaks", len(tweaks))
	return nil
}

func OverWriteTweaks(tweaks []types.Tweak) error {
	var tweaksToOverwrite []types.Tweak
	for _, tweak := range tweaks {
		pairs, err := retrieveManyByBlockHashAndTxid(TweaksDB, tweak.BlockHash, tweak.Txid, types.PairFactoryTweak)
		if err != nil && !errors.Is(err, NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			return err
		} else if err != nil && errors.Is(err, NoEntryErr{}) {
			// This should not happen because the overwrites are computed from remaining UTXOs.
			// Getting this error would mean that we have UTXOs without corresponding tweaks in the DB
			common.ErrorLogger.Println("no entries for a tweak were found. this should not happen")
			return err // keep this as an error. if this happens we have to know
		}

		// this will be removed as we still test, see below
		if len(pairs) != 1 {
			// this scenario should never happen. The database should not have >1 entries for one transaction. <1 (0) should give no entry error
			// prev
			common.ErrorLogger.Printf("%+v", pairs)
			common.ErrorLogger.Println("number of tweaks was not exactly 1")
			return errors.New("number of tweaks was not exactly 1")
		}

		var result types.Tweak
		// Convert Pair to a Tweak and assign it to the new slice
		if pairPtr, ok := pairs[0].(*types.Tweak); ok {
			result = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pairs[0])
			panic("wrong pair struct returned")
		}
		tweak.TweakData = result.TweakData

		tweaksToOverwrite = append(tweaksToOverwrite, tweak)

	}

	err := InsertBatchTweaks(tweaksToOverwrite)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return err
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
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = &pairCopy
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

func DustOverwriteRoutine() error {
	// todo has some issues remaining biggest remaining UTXOs
	iter := TweaksDB.NewIterator(nil, nil)
	defer iter.Release()

	var tweaksForBatchInsert []types.Tweak
	counter := 0
	for iter.Next() {
		counter++
		// Deserialize data first
		tweak := types.Tweak{}
		err := tweak.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		err = tweak.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		utxos, err := FetchByBlockHashAndTxidUTXOs(tweak.BlockHash, tweak.Txid)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
		// we insert a fake spentUTXO such that the highest of the remaining will be taken.
		highestValue, err := types.FindBiggestRemainingUTXO(types.UTXO{Value: math.MaxUint64}, utxos)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
		// todo highestValue might be nil here
		tweak.HighestValue = *highestValue
		tweaksForBatchInsert = append(tweaksForBatchInsert, tweak)
		if counter%2_500 == 0 {
			common.InfoLogger.Println("Inserting for", counter)
			// we use insert instead of overwrite because we already have all the information ready
			err = InsertBatchTweaks(tweaksForBatchInsert)
			if err != nil {
				common.ErrorLogger.Println(err)
				return err
			}
			tweaksForBatchInsert = []types.Tweak{}
		}
	}

	// insert the remaining tweaks
	err := InsertBatchTweaks(tweaksForBatchInsert)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return err
}

func FetchByBlockHashDustLimitTweaks(blockHash string, dustLimit uint64) ([]types.Tweak, error) {
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	iter := TweaksDB.NewIterator(util.BytesPrefix(blockHashBytes), nil)
	defer iter.Release()
	var results []types.Tweak

	for iter.Next() {
		tweak := types.Tweak{}
		// Deserialize data first
		err = tweak.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		if tweak.HighestValue >= dustLimit {
			err = tweak.DeSerialiseKey(iter.Key())
			if err != nil {
				common.ErrorLogger.Println(err)
				return nil, err
			}
			results = append(results, tweak)
		}
	}

	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return results, err
}
