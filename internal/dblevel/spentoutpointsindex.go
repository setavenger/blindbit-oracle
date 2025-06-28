package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertSpentOutpointsIndex(pair *types.SpentOutpointsIndex) error {
	err := insertSimple(SpentOutpointsIndexDB, pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting spent outpoints index")
		return err
	}
	logging.L.Trace().Msg("spent outpoints index inserted")
	return nil
}

func FetchByBlockHashSpentOutpointIndex(blockHash [32]byte) (*types.SpentOutpointsIndex, error) {
	var pair types.SpentOutpointsIndex
	err := retrieveByBlockHash(SpentOutpointsIndexDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		logging.L.Err(err).Msg("error fetching spent outpoints index")
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		logging.L.Err(err).Msg("error fetching spent outpoints index")
		return nil, err
	}
	return &pair, nil
}

// FetchAllTweakIndices returns all types.TweakIndex in the DB
func FetchAllSpenOutpointsIndices() ([]types.SpentOutpointsIndex, error) {
	pairs, err := retrieveAll(SpentOutpointsIndexDB, types.PairFactorySpentOutpointsIndex)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all spent outpoints indices")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.SpentOutpointsIndex, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.SpentOutpointsIndex); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Panic().Err(err).Msg("wrong pair struct returned")
		}
	}
	return result, err
}
