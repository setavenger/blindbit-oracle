package dblevel

import (
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertSpentOutpointsFilter(pair types.Filter) error {
	err := insertSimple(SpentOutpointsFilterDB, &pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting spent outpoints filter")
		return err
	}
	logging.L.Trace().Msg("Taproot Filter inserted")
	return nil
}

func FetchByBlockHashSpentOutpointsFilter(blockHash [32]byte) (types.Filter, error) {
	var pair types.Filter
	err := retrieveByBlockHash(SpentOutpointsFilterDB, blockHash, &pair)
	if err != nil {
		logging.L.Err(err).Msg("error fetching spent outpoints filter")
		return types.Filter{}, err
	}
	return pair, nil
}

// FetchAllFilters returns all types.Filter in the DB
func FetchAllSpentOutpointsFilters() ([]types.Filter, error) {
	pairs, err := retrieveAll(SpentOutpointsFilterDB, types.PairFactoryFilter)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all spent outpoints filters")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.Filter, len(pairs))
	// Convert each Pair to a Filter and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.Filter); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Panic().Err(err).Msg("wrong pair struct returned")
		}
	}
	return result, err
}
