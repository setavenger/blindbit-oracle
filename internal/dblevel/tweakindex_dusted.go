package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertTweakIndexDust(pair *types.TweakIndexDust) error {
	err := insertSimple(TweakIndexDustDB, pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting tweak index with dust filter")
		return err
	}
	logging.L.Debug().Msg("tweak index with dust filter inserted")
	return nil
}

func FetchByBlockHashTweakIndexDust(blockHash string) (*types.TweakIndexDust, error) {
	var pair types.TweakIndexDust
	err := retrieveByBlockHash(TweakIndexDustDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		logging.L.Err(err).Msg("error fetching tweak index with dust filter")
		return nil, err
	} else if err != nil && errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		logging.L.Err(err).Msg("error fetching tweak index with dust filter")
		return nil, err
	}
	// todo this probably does not need to be a pointer
	return &pair, nil
}

// FetchAllTweakIndicesDust returns all types.TweakIndexDust in the DB
func FetchAllTweakIndicesDust() ([]types.TweakIndexDust, error) {
	pairs, err := retrieveAll(TweakIndexDustDB, types.PairFactoryTweakIndexDust)
	if err != nil {
		logging.L.Err(err).Msg("error fetching all tweak indices with dust filter")
		return nil, err
	}
	if len(pairs) == 0 {
		logging.L.Warn().Msg("Nothing returned")
		return nil, NoEntryErr{}
	}

	result := make([]types.TweakIndexDust, len(pairs))
	// Convert each Pair to a TweakIndex and assign it to the new slice
	for i, pair := range pairs {
		if pairPtr, ok := pair.(*types.TweakIndexDust); ok {
			result[i] = *pairPtr
		} else {
			logging.L.Panic().Err(err).Msg("wrong pair struct returned")
		}
	}
	return result, err
}
