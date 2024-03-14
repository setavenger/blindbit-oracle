package dblevel

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
)

func InsertUTXOs(utxos []types.UTXO) error {
	common.DebugLogger.Println("Inserting UTXOs...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairs[i] = &pair
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
	common.DebugLogger.Println("Deleting UTXOs...")
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairs[i] = &pair
	}
	err := deleteBatch(UTXOsDB, pairs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Printf("Deleted %d UTXOs\n", len(utxos))
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
