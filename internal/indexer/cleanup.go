package indexer

import (
	"context"
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

// Cleamup iterates from config syncstartheight to tip and
// checks for missing blocks to pull those and patch missing block data
func (b *Builder) DBIntegrityCheck(ctx context.Context) error {
	_, syncTip, err := b.store.GetChainTip()
	if err != nil {
		return err
	}

	logging.L.Info().
		Uint32("config_sync_start_height", config.SyncStartHeight).
		Uint32("sync_tip", syncTip).
		Msg("syncing heights")

	startHeight, endHeight := config.SyncStartHeight, syncTip

	ranges, err := b.identifyGapRanges(startHeight, endHeight)
	if err != nil {
		logging.L.Err(err).Msg("failed to predetermine gap height ranges")
		return err
	}

	if len(ranges) > 0 {
		logging.L.Info().Any("ranges", ranges).Msg("identified ranges")
	} else {
		logging.L.Info().
			Uint32("start_height", startHeight).
			Uint32("end_height", endHeight).
			Msg("no gaps identified for block range")
	}

	for i := range ranges {
		start, end := ranges[i].Start, ranges[i].End
		logging.L.Debug().Msgf("filling gap %d -> %d", start, end)
		err = b.SyncBlocks(b.ctx, int64(start), int64(end))
		if err != nil {
			return err
		}
	}

	// for height := config.SyncStartHeight; height <= syncTip; height++ {
	// 	// todo: turn into more efficient sync blocks (concurrent)
	// 	// if somehow big gaps become apparent
	// 	blockhash, err := b.store.GetBlockHashByHeight(height)
	// 	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
	// 		return err
	// 	}
	//
	// 	if blockhash != nil {
	// 		// block is already processed
	// 		continue
	// 	}
	//
	// 	logging.L.Info().
	// 		Uint32("height", height).
	// 		Msg("block is missing, trying to process block")
	// err = b.singleProcessBlock(b.ctx, height)
	// 	if err != nil {
	// 		logging.L.Err(err).Uint32("height", height).Msg("failed to process block")
	// 		return err
	// 	}
	// }

	err = b.store.FlushBatch(true)
	if err != nil {
		logging.L.Err(err).Msg("failed flushing batch")
		return err
	}

	return nil
}

type HeightRange struct {
	Start uint32
	End   uint32
}

func (b *Builder) identifyGapRanges(
	startHeight, endHeight uint32,
) ([]HeightRange, error) {
	var allRanges []HeightRange
	var currentRange *HeightRange

	for height := startHeight; height <= endHeight; height++ {
		// todo: turn into more efficient sync blocks (concurrent)
		// if somehow big gaps become apparent
		blockhash, err := b.store.GetBlockHashByHeight(height)
		if err != nil && !errors.Is(err, pebble.ErrNotFound) {
			return nil, err
		}

		if blockhash != nil {
			// block is already processed
			if currentRange != nil {
				if currentRange.End == 0 {
					// todo: until we can do point pulls
					// sync range needs end and it's easier to define here
					currentRange.End = currentRange.Start
				}
				allRanges = append(allRanges, *currentRange)
				currentRange = nil
			}
			continue
		}

		if currentRange == nil {
			currentRange = new(HeightRange)
			currentRange.Start = height
			continue
		}

		currentRange.End = height
	}

	return allRanges, nil
}
