package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"

	"github.com/syndtr/goleveldb/leveldb/util"
)

func InsertUTXOs(utxos []types.UTXO) error {
	// common.DebugLogger.Println("Inserting UTXOs...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = &pairCopy
	}

	err := insertBatch(UTXOsDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Inserted %d UTXOs", len(utxos))
	return nil
}

func FetchByBlockHashUTXOs(blockHash string) ([]types.UTXO, error) {
	//common.InfoLogger.Println("Fetching UTXOs")
	pairs, err := retrieveManyByBlockHash(UTXOsDB, blockHash, types.PairFactoryUTXO)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, NoEntryErr{}
	}

	result := make([]types.UTXO, len(pairs))
	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.UTXO); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	//common.InfoLogger.Printf("Fetched %d UTXOs\n", len(result))

	return result, nil
}

func FetchByBlockHashAndTxidUTXOs(blockHash, txid string) ([]types.UTXO, error) {
	//common.InfoLogger.Println("Fetching UTXOs")
	pairs, err := retrieveManyByBlockHashAndTxid(UTXOsDB, blockHash, txid, types.PairFactoryUTXO)
	if err != nil {
		if errors.Is(err, NoEntryErr{}) {
			// don't print if it's a no entry error
			return nil, err
		}
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, NoEntryErr{}
	}

	result := make([]types.UTXO, len(pairs))
	// Convert each Pair to a UTXO and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.UTXO); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	//common.InfoLogger.Printf("Fetched %d UTXOs\n", len(result))

	return result, nil
}

func DeleteBatchUTXOs(utxos []types.UTXO) error {
	// common.DebugLogger.Println("Deleting UTXOs...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = &pairCopy
	}
	err := deleteBatch(UTXOsDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	// common.DebugLogger.Printf("Deleted %d UTXOs\n", len(utxos))
	return nil
}

// FetchAllUTXOs returns all types.UTXO in the DB
func FetchAllUTXOs() ([]types.UTXO, error) {
	pairs, err := retrieveAll(UTXOsDB, types.PairFactoryUTXO)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if len(pairs) == 0 {
		common.WarningLogger.Println("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.UTXO, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.UTXO); ok {
			result[i] = *pairPtr
		} else {
			common.ErrorLogger.Printf("%+v\n", pair)
			panic("wrong pair struct returned")
		}
	}
	return result, err
}

// PruneUTXOs iterates over a set of utxos according to a prefix (all if set to nil).
// The function checks whether the utxos are eligible for removal.
func PruneUTXOs(prefix []byte) error {
	iter := UTXOsDB.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	// totalSet is for the final batch deletion
	var totalSetToDelete []types.UTXO

	var lastTxid string
	var canBeRemoved = true
	var currentSet []types.UTXO

	var err error

	for iter.Next() {

		var value types.UTXO

		err = value.DeSerialiseKey(iter.Key())
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		if lastTxid == "" {
			lastTxid = value.Txid
		}

		if !canBeRemoved && value.Txid == lastTxid {
			continue
		}

		err = value.DeSerialiseData(iter.Value())
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		if value.Spent {
			currentSet = append(currentSet, value)
		} else {
			canBeRemoved = false
		}

		if value.Txid != lastTxid {
			// delete the current set of UTXOs if eligible

			// do deletion
			if lastTxid != "" && canBeRemoved {
				totalSetToDelete = append(totalSetToDelete, currentSet...)
				// common.DebugLogger.Printf("Added %d UTXOs for deletion - %s\n", len(currentSet), lastTxid)
			}

			// reset state
			currentSet = nil
			canBeRemoved = true
		}
		lastTxid = value.Txid
	}

	// Handle the last batch of UTXOs after the loop
	if canBeRemoved && len(currentSet) > 0 {
		totalSetToDelete = append(totalSetToDelete, currentSet...)
		// common.DebugLogger.Printf("Added %d UTXOs for deletion - %s\n", len(currentSet), lastTxid)
	}
	err = iter.Error()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = DeleteBatchUTXOs(totalSetToDelete)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return nil
}
