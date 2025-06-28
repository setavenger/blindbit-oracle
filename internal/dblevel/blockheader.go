package dblevel

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func InsertBlockHeader(pair types.BlockHeader) error {
	err := insertSimple(HeadersDB, &pair)
	if err != nil {
		logging.L.Err(err).Msg("error inserting block header")
		return err
	}
	logging.L.Trace().Msg("block_header inserted")
	return nil
}

func FetchByBlockHashBlockHeader(blockHash [32]byte) (*types.BlockHeader, error) {
	var pair types.BlockHeader
	err := retrieveByBlockHash(HeadersDB, blockHash, &pair)
	if err != nil && !errors.Is(err, NoEntryErr{}) {
		logging.L.Err(err).Msg("error fetching block header")
		return nil, err
	} else if errors.Is(err, NoEntryErr{}) { // todo why do we return the error anyways?
		// todo find good solution, muted because it will show up for every pull we make
		// logging.L.Err(err).Msg("error fetching block header")
		return nil, err
	}
	return &pair, nil
}
