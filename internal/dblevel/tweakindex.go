package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertTweakIndex(pair *types.TweakIndex) error {
	err := insertSimple(TweakIndexDB, pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting tweak index")
		return err
	}
	logging.L.Trace().Msg("tweak index inserted")
	return nil
}

func FetchByBlockHashTweakIndex(blockHash [32]byte) (*types.TweakIndex, error) {
	var pair types.TweakIndex
	err := retrieveByBlockHash(TweakIndexDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		logging.L.Err(err).Msg("error fetching tweak index")
		return nil, err
	} else if err != nil && errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		logging.L.Err(err).Msg("error fetching tweak index")
		return nil, err
	}
	// todo this probably does not need to be a pointer
	return &pair, nil
}

// FetchAllTweakIndices returns all types.TweakIndex in the DB
func FetchAllTweakIndices() ([]types.TweakIndex, error) {
	pairs, err := retrieveAll(TweakIndexDB, types.PairFactoryTweakIndex)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all tweak indices")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.TweakIndex, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.TweakIndex); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Panic().Err(err).Msg("wrong pair struct returned")
		}
	}
	return result, err
}
