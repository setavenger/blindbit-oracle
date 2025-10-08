package indexer

import (
	"context"
	"errors"
	"fmt"
	"sync"

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

	// Check for missing static indexes
	// logging.L.Info().Msg("Checking static index integrity...")
	// err = b.checkStaticIndexIntegrity(startHeight, endHeight)
	// if err != nil {
	// 	return err
	// }

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

// checkStaticIndexIntegrity checks if static indexes exist for all blocks in the range
func (b *Builder) checkStaticIndexIntegrity(startHeight, endHeight uint32) error {
	var missingIndexes []uint32

	for height := startHeight; height <= endHeight; height++ {
		blockhash, err := b.store.GetBlockHashByHeight(height)
		if err != nil {
			if errors.Is(err, pebble.ErrNotFound) {
				continue // Block doesn't exist, skip
			}
			return err
		}

		// Check if static indexes exist for this block
		hasTweaks, err := b.store.KeyExistsStaticTweaks(blockhash)
		if err != nil {
			return err
		}

		hasOutputs, err := b.store.KeyExistsStaticOutputs(blockhash)
		if err != nil {
			return err
		}

		hasUnspentFilter, err := b.store.KeyExistsStaticTaprootUnspentFilter(blockhash)
		if err != nil {
			return err
		}

		// Note: spent outpoints are now accelerator indexes, not static indexes

		// If any static index is missing, add to list
		if !hasTweaks || !hasOutputs || !hasUnspentFilter {
			missingIndexes = append(missingIndexes, height)
		}
	}

	if len(missingIndexes) > 0 {
		logging.L.Info().
			Int("missing_count", len(missingIndexes)).
			Uint32("start_height", startHeight).
			Uint32("end_height", endHeight).
			Msg("Found blocks with missing static indexes, rebuilding...")

		// Rebuild static indexes for blocks that are missing them using parallel workers
		rebuildErr := b.rebuildStaticIndexesParallel(missingIndexes)
		if rebuildErr != nil {
			logging.L.Warn().Err(rebuildErr).Msg("failed to rebuild static indexes in parallel")
		}

		logging.L.Info().
			Int("rebuilt_count", len(missingIndexes)).
			Msg("Static index integrity check completed - rebuilt missing indexes")
	} else {
		logging.L.Info().
			Uint32("start_height", startHeight).
			Uint32("end_height", endHeight).
			Msg("Static index integrity check completed - all indexes present")
	}

	return nil
}

// rebuildStaticIndexesParallel rebuilds static indexes for multiple blocks in parallel
func (b *Builder) rebuildStaticIndexesParallel(missingIndexes []uint32) error {
	if len(missingIndexes) == 0 {
		return nil
	}

	// Use configurable number of workers, but cap at number of blocks
	numWorkers := config.MaxParallelTweakComputations
	if numWorkers > len(missingIndexes) {
		numWorkers = len(missingIndexes)
	}

	logging.L.Info().
		Int("total_blocks", len(missingIndexes)).
		Int("num_workers", numWorkers).
		Msg("Starting parallel static index rebuild")

	// Create channels for work distribution
	heightChan := make(chan uint32, len(missingIndexes))
	resultChan := make(chan error, len(missingIndexes))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for height := range heightChan {
				blockhash, err := b.store.GetBlockHashByHeight(height)
				if err != nil {
					logging.L.Warn().Err(err).Uint32("height", height).Int("worker", workerID).Msg("failed to get blockhash for static index rebuild")
					resultChan <- err
					continue
				}

				logging.L.Debug().Uint32("height", height).Int("worker", workerID).Msg("rebuilding static indexes for block")
				err = b.store.ReindexBlockWithOptions(blockhash, false, true)
				if err != nil {
					logging.L.Warn().Err(err).Uint32("height", height).Int("worker", workerID).Msg("failed to rebuild static indexes")
					resultChan <- err
					continue
				}

				resultChan <- nil
			}
		}(i)
	}

	// Send work to workers
	go func() {
		defer close(heightChan)
		for _, height := range missingIndexes {
			heightChan <- height
		}
	}()

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var errors []error
	for err := range resultChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		logging.L.Warn().
			Int("error_count", len(errors)).
			Int("total_blocks", len(missingIndexes)).
			Msg("Some static index rebuilds failed")
		return fmt.Errorf("failed to rebuild static indexes for %d blocks", len(errors))
	}

	logging.L.Info().
		Int("successful_blocks", len(missingIndexes)).
		Msg("Parallel static index rebuild completed successfully")

	return nil
}
