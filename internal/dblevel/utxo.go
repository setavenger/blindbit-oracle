package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func InsertUTXOs(utxos []*types.UTXO) error {
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = pairCopy
	}

	err := insertBatch(UTXOsDB, pairs)
	if err != nil {
		logging.L.Err(err).Msg("error inserting utxos")
		return err
	}
	logging.L.Trace().Msgf("Inserted %d UTXOs", len(utxos))
	return nil
}

func FetchByBlockHashUTXOs(blockHash [32]byte) ([]types.UTXO, error) {
	pairs, err := retrieveManyByBlockHash(UTXOsDB, blockHash, types.PairFactoryUTXO)
	if err != nil {
		logging.L.Err(err).Msg("error fetching utxos by block hash")
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
			logging.L.Err(err).Msg("wrong pair struct returned")
			panic("wrong pair struct returned")
		}
	}

	return result, nil
}

func FetchByBlockHashAndTxidUTXOs(blockHash, txid [32]byte) ([]types.UTXO, error) {
	pairs, err := retrieveManyByBlockHashAndTxid(UTXOsDB, blockHash, txid, types.PairFactoryUTXO)
	if err != nil {
		if errors.Is(err, NoEntryErr{}) {
			// don't print if it's a no entry error
			return nil, err
		}
		logging.L.Err(err).Msg("error fetching utxos by block hash and txid")
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
			logging.L.Err(err).Msg("wrong pair struct returned")
			panic("wrong pair struct returned")
		}
	}

	return result, nil
}

func DeleteBatchUTXOs(utxos []types.UTXO) error {
	// Create a slice of types.Pair with the same length as pairs
	pairs := make([]types.Pair, len(utxos))

	// Convert each UTXO to a Pair and assign it to the new slice
	for i, pair := range utxos {
		pairCopy := pair // Create a new variable that is a copy of pair
		pairs[i] = &pairCopy
	}
	err := deleteBatch(UTXOsDB, pairs)
	if err != nil {
		logging.L.Err(err).Msg("error deleting utxos")
		return err
	}
	return nil
}

// FetchAllUTXOs returns all types.UTXO in the DB
func FetchAllUTXOs() ([]types.UTXO, error) {
	pairs, err := retrieveAll(UTXOsDB, types.PairFactoryUTXO)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all utxos")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.UTXO, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.UTXO); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Panic().Err(err).Any("pair", pair).Msg("wrong pair struct returned")
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

	var lastTxid [32]byte
	var canBeRemoved = true
	var currentSet []types.UTXO

	var err error

	for iter.Next() {

		var value types.UTXO

		err = value.DeSerialiseKey(iter.Key())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising key")
			return err
		}

		if lastTxid == [32]byte{} {
			lastTxid = value.Txid
		}

		if !canBeRemoved && value.Txid == lastTxid {
			continue
		}

		err = value.DeSerialiseData(iter.Value())
		if err != nil {
			logging.L.Err(err).Msg("error deserialising data")
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
			if lastTxid != [32]byte{} && canBeRemoved {
				totalSetToDelete = append(totalSetToDelete, currentSet...)
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
	}
	err = iter.Error()
	if err != nil {
		logging.L.Err(err).Msg("error iterating over utxos")
		return err
	}

	err = DeleteBatchUTXOs(totalSetToDelete)
	if err != nil {
		logging.L.Err(err).Msg("error deleting utxos")
		return err
	}
	return nil
}
