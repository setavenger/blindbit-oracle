package dbpebble

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// BuildStaticIndexing pulls the entire DB and rewrites as static indexes.
// If force is true, it will rebuild the static data for all blocks.
// By default it only builds statics from config.SyncStartHeight to tip.
//
// Tweaks:
//
// key: blockhash   value: <33byte><33byte>...<33byte>
//
// Outputs:
//
// key: blockhash   value: <76byte><76byte>...<76byte>
//
// Outputs binary serialisation is defined in internal/database/serialisation.go
func (s *Store) BuildStaticIndexing(force bool) error {
	blockhashTip, heightTip, err := s.GetChainTip()
	if err != nil {
		return err
	}
	blockhashStart, heightStart, err := s.FirstBlock()
	if err != nil {
		return err
	}

	// heightStart = 100_000
	// heightTip = 260_000

	totalBlocks := heightTip - heightStart + 1
	logging.L.Info().
		Uint32("height_start", heightStart).
		Uint32("height_tip", heightTip).
		Uint32("total_blocks", totalBlocks).
		Msgf("Building static indexes from %d -> %d (%d blocks)", heightStart, heightTip, totalBlocks)
	logging.L.Debug().
		Hex("blockhash_start", utils.ReverseBytesCopy(blockhashStart)).
		Hex("blockhash_tip", utils.ReverseBytesCopy(blockhashTip)).
		Uint32("height_start", heightStart).
		Uint32("height_tip", heightTip).
		Msg("indexing details")

	// if config.Chain == config.Mainnet {
	// 	if heightStart < 700_000 {
	// 		heightStart = 700_000
	// 	}
	// }

	blockhashChan, err := s.ChainIterator(true)
	if err != nil {
		return err
	}
	errChan := make(chan error)
	var wg sync.WaitGroup
	var blockCounter int64
	var lastProgressTime time.Time
	var startTime time.Time

	// Start worker goroutines
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for blockhash := range blockhashChan {
				count := atomic.AddInt64(&blockCounter, 1)

				// Set start time on first block
				if count == 1 {
					startTime = time.Now()
				}

				err := s.ReindexBlock(blockhash, force)
				if err != nil {
					logging.L.Err(err).
						Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
						Msg("static indexes failed")
					errChan <- err
					return
				}

				// Enhanced progress reporting
				now := time.Now()
				shouldReport := count%50 == 0 || now.Sub(lastProgressTime) >= 10*time.Second

				if shouldReport {
					height, ok, err := s.heightIfOnBestChain(blockhash)
					if err != nil {
						logging.L.Err(err).
							Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
							Msg("failed to get height if on best chain")
						errChan <- err
						return
					}
					if !ok {
						logging.L.Warn().
							Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
							Msg("block not on best chain")
						continue
					}

					// Calculate progress and ETA
					progress := float64(count) / float64(totalBlocks) * 100
					remainingBlocks := totalBlocks - uint32(count)

					// Calculate ETA
					elapsed := now.Sub(startTime)
					blocksPerSecond := float64(count) / elapsed.Seconds()
					var eta time.Duration
					if blocksPerSecond > 0 {
						eta = time.Duration(float64(remainingBlocks)/blocksPerSecond) * time.Second
					}

					// Get current blockchain tip for context
					// Note: Removed chainInfo call to avoid circular dependency
					// The progress reporting will work without this information

					logging.L.Info().
						Int64("processed_blocks", count).
						Uint32("current_height", height).
						Uint32("total_blocks", totalBlocks).
						Float64("progress_percent", progress).
						Uint32("remaining_blocks", remainingBlocks).
						Float64("blocks_per_second", blocksPerSecond).
						Dur("elapsed_time", elapsed).
						Dur("eta", eta).
						Msgf("Static index progress: %d/%d blocks (%.1f%%) - processing height %d - ETA: %v", count, totalBlocks, progress, height, eta)

					lastProgressTime = now
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	select {
	case err := <-errChan:
		logging.L.Err(err).Msg("ended with err")
		return err
	default:
		// No errors
	}

	close(errChan)

	return nil
}

// BuildStaticIndexesForBlock builds static indexes for a single block with integrity checks
// This function builds static indexes if they are missing, but does not force rebuild existing ones
func (s *Store) BuildStaticIndexesForBlock(blockhash []byte) error {
	// Check which static indexes exist
	existsTweaks, err := s.KeyExistsStaticTweaks(blockhash)
	if err != nil {
		return err
	}
	existsOutputs, err := s.KeyExistsStaticOutputs(blockhash)
	if err != nil {
		return err
	}
	existsUnspentFilter, err := s.KeyExistsStaticTaprootUnspentFilter(blockhash)
	if err != nil {
		return err
	}

	// If all static indexes exist, nothing to do
	if existsTweaks && existsOutputs && existsUnspentFilter {
		logging.L.Trace().
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("all static indexes exist, skipping build")
		return nil
	}

	logging.L.Debug().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Bool("existsTweaks", existsTweaks).
		Bool("existsOutputs", existsOutputs).
		Bool("existsUnspentFilter", existsUnspentFilter).
		Msg("building missing static indexes")

	// Fetch data for indexes that need building
	var outputs []*database.Output
	var tweaks []*database.TweakRow
	var outputDBValue []byte
	var tweaksDBValue []byte
	var unspentFilter []byte

	if !existsTweaks || !existsOutputs || !existsUnspentFilter {
		outputs, err = s.FetchOutputsAll(blockhash, math.MaxUint32)
		if err != nil {
			return err
		}

		tweaks, err = s.TweaksForBlockAll(blockhash)
		if err != nil {
			return err
		}

		// Convert to static DB values
		outputDBValue = convertOutputsToStaticDBValue(outputs)
		tweaksDBValue = convertTweakRowToStaticDBValue(tweaks)

		unspentFilter, err = buildUnspentFilter(blockhash, outputs)
		if err != nil {
			return err
		}
	}

	// Use existing data for indexes that don't need rebuilding
	if existsTweaks {
		existingTweaks, err := s.FetchTweaksStatic(blockhash)
		if err != nil {
			logging.L.Warn().Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to fetch existing tweaks, will rebuild")
			if tweaks == nil {
				tweaks, err = s.TweaksForBlockAll(blockhash)
				if err != nil {
					return err
				}
			}
			tweaksDBValue = convertTweakRowToStaticDBValue(tweaks)
		} else {
			// Convert existing tweaks to DB format
			tweaksDBValue = make([]byte, 33*len(existingTweaks))
			for i, tweak := range existingTweaks {
				copy(tweaksDBValue[i*33:(i+1)*33], tweak)
			}
		}
	}

	if existsOutputs {
		existingOutputs, err := s.FetchOutputsStatic(blockhash)
		if err != nil {
			logging.L.Warn().Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to fetch existing outputs, will rebuild")
			if outputs == nil {
				outputs, err = s.FetchOutputsAll(blockhash, math.MaxUint32)
				if err != nil {
					return err
				}
			}
			outputDBValue = convertOutputsToStaticDBValue(outputs)
		} else {
			// Convert existing outputs to DB format
			outputDBValue = convertOutputsToStaticDBValue(existingOutputs)
		}
	}

	if existsUnspentFilter {
		existingUnspentFilter, err := s.FetchTaprootUnspentFilter(blockhash)
		if err != nil {
			logging.L.Warn().Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to fetch existing unspent filter, will rebuild")
			if outputs == nil {
				outputs, err = s.FetchOutputsAll(blockhash, math.MaxUint32)
				if err != nil {
					return err
				}
			}
			unspentFilter, err = buildUnspentFilter(blockhash, outputs)
			if err != nil {
				return err
			}
		} else {
			unspentFilter = existingUnspentFilter
		}
	}

	err = s.finishBlockStatics(blockhash, tweaksDBValue, outputDBValue, unspentFilter)
	if err != nil {
		return err
	}

	logging.L.Debug().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Msg("static indexes built successfully")

	return nil
}

// BuildAcceleratorIndexesForBlock builds accelerator indexes (like compute index) for a block
// These are indexes that are built on-demand and used for efficient scanning
func (s *Store) BuildAcceleratorIndexesForBlock(blockhash []byte) error {
	// Check if compute index exists
	existsComputeIndex, err := s.KeyExistsComputeIndex(blockhash)
	if err != nil {
		return err
	}

	// If compute index exists, nothing to do
	if existsComputeIndex {
		logging.L.Trace().
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("compute index exists, skipping accelerator index build")
		return nil
	}

	logging.L.Debug().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Msg("building accelerator indexes")

	// Build compute index
	height, ok, err := s.heightIfOnBestChain(blockhash)
	if err != nil {
		return err
	}
	if !ok {
		logging.L.Warn().
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("block not on best chain, skipping accelerator index build")
		return nil
	}

	computeIndexes, err := s.BuildComputeIndexForHeight(height)
	if err != nil {
		logging.L.Err(err).
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Uint32("height", height).
			Msg("failed to build compute index")
		return err
	}

	err = s.FinishComputeIndex(height, computeIndexes)
	if err != nil {
		logging.L.Err(err).
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Uint32("height", height).
			Msg("failed to finish compute index")
		return err
	}

	logging.L.Debug().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Uint32("height", height).
		Int("compute_index_count", len(computeIndexes)).
		Msg("accelerator indexes built successfully")

	return nil
}

// ReindexBlock rebuilds static indexes for a block only if explicitly requested with force=true
// This function should only be called with user intention, not automatically
func (s *Store) ReindexBlock(blockhash []byte, force bool) error {
	if !force {
		logging.L.Warn().
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("ReindexBlock called without force=true, skipping rebuild")
		return nil
	}

	logging.L.Info().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Msg("force rebuilding static indexes (user requested)")

	// Force rebuild all static indexes
	return s.ReindexBlockWithOptions(blockhash, true, false)
}

// ReindexBlockWithOptions provides fine-grained control over which indexes to rebuild
// This function should only be called with explicit user intention (force flags)
// forceAll: if true, rebuilds all indexes regardless of whether they exist
// forceInexpensive: if true, rebuilds cheap indexes (tweaks, outputs, unspent filter) even if they exist
func (s *Store) ReindexBlockWithOptions(blockhash []byte, forceAll bool, forceInexpensive bool) error {
	logging.L.Info().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Bool("forceAll", forceAll).
		Bool("forceInexpensive", forceInexpensive).
		Msg("force rebuilding static indexes (user requested)")

	// For force rebuilds, we rebuild everything regardless of existence
	// This is simpler and more predictable than the complex logic we had before
	outputs, err := s.FetchOutputsAll(blockhash, math.MaxUint32)
	if err != nil {
		return err
	}

	tweaks, err := s.TweaksForBlockAll(blockhash)
	if err != nil {
		return err
	}

	// Convert to static DB values
	outputDBValue := convertOutputsToStaticDBValue(outputs)
	tweaksDBValue := convertTweakRowToStaticDBValue(tweaks)

	unspentFilter, err := buildUnspentFilter(blockhash, outputs)
	if err != nil {
		return err
	}

	err = s.finishBlockStatics(blockhash, tweaksDBValue, outputDBValue, unspentFilter)
	if err != nil {
		return err
	}

	logging.L.Info().
		Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
		Msg("force rebuild completed successfully")

	return nil
}

func (s *Store) finishBlockStatics(
	blockhash, tweaks, outputs, unspentFilter []byte,
) error {
	// Use a more efficient approach: queue the write operation
	// This reduces lock contention by minimizing the time spent in the critical section
	s.batchSync.Lock()
	defer s.batchSync.Unlock()

	err := attachStaticsToBatch(s.dbBatch, blockhash, tweaks, outputs, unspentFilter)
	if err != nil {
		return err
	}

	s.batchCounter++
	// Increase batch size to reduce commit frequency and improve throughput
	if s.batchCounter >= 500 { // Increased from 200 to 500
		err = s.commitBatch(false)
		if err != nil {
			return err
		}
	}
	return nil
}

func attachStaticsToBatch(
	batch *pebble.Batch,
	blockhash, tweaks, outputs, unspentFilter []byte,
) error {
	if err := batch.Set(KeyTweaksStatic(blockhash), tweaks, nil); err != nil {
		return err
	}

	if err := batch.Set(KeyKUTXOsStatic(blockhash), outputs, nil); err != nil {
		return err
	}

	if err := batch.Set(KeyTaprootUnspentFilter(blockhash), unspentFilter, nil); err != nil {
		return err
	}

	// Note: spent outpoints are now accelerator indexes, not static indexes
	return nil
}

func convertTweakRowToStaticDBValue(tweaks []*database.TweakRow) (out []byte) {
	out = make([]byte, 33*len(tweaks))
	for i := range tweaks {
		copy(out[i*33:(i+1)*33], tweaks[i].Tweak[:])
	}
	return
}

func convertOutputsToStaticDBValue(outputs []*database.Output) []byte {
	outLen := database.OutputBinLength
	out := make([]byte, database.OutputBinLength*len(outputs))
	for i := range outputs {
		copy(out[i*outLen:(i+1)*outLen], outputs[i].BinarySerialisation())
	}
	return out
}

func buildUnspentFilter(blockhash []byte, outputs []*database.Output) ([]byte, error) {
	taprootOutputs := make([][]byte, len(outputs))
	for i := range outputs {
		taprootOutputs[i] = outputs[i].Pubkey
	}
	return buildTaprootPubkeyFilter(blockhash, taprootOutputs)
}

// buildTaprootPubkeyFilter creates the taproot only filter (local version to avoid circular dependency)
func buildTaprootPubkeyFilter(blockhash []byte, taprootOutputs [][]byte) ([]byte, error) {
	// blockhash is already reversed
	c := chainhash.Hash{}
	err := c.SetBytes(blockhash)

	if err != nil {
		logging.L.Fatal().
			Err(err).Hex("blockhash", blockhash).
			Msg("failed to set block hash")
		return nil, err
	}
	key := builder.DeriveKey(&c)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutputs)
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", blockhash).Msg("failed to build GCS filter")
		return nil, err
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", blockhash).Msg("failed to get NBytes")
		return nil, err
	}

	return nBytes, nil
}

// Note: spent outpoints functions removed - they are now accelerator indexes, not static indexes
