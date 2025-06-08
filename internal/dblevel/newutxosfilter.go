package dblevel

import (
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertNewUTXOsFilter(pair types.Filter) error {
	err := insertSimple(NewUTXOsFiltersDB, &pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting new utxos filter")
		return err
	}
	logging.L.Trace().Msg("Taproot Filter inserted")
	return nil
}

func FetchByBlockHashNewUTXOsFilter(blockHash string) (types.Filter, error) {
	var pair types.Filter
	err := retrieveByBlockHash(NewUTXOsFiltersDB, blockHash, &pair)
	if err != nil {
		logging.L.Err(err).Msg("error fetching new utxos filter by block hash")
		return types.Filter{}, err
	}
	return pair, nil
}

// FetchAllFilters returns all types.Filter in the DB
func FetchAllNewUTXOsFilters() ([]types.Filter, error) {
	pairs, err := retrieveAll(NewUTXOsFiltersDB, types.PairFactoryFilter)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all new utxos filters")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.Filter, len(pairs))
	// Convert each Pair to a Filter and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.Filter); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Err(err).Any("pair", pair).Msg("wrong pair struct returned")
			panic("wrong pair struct returned")
		}
	}
	return result, err
}
