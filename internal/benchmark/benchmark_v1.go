package benchmark

import (
	"sync"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/networking"
)

// BenchmarkV1 runs the v1 HTTP API benchmark using the existing ClientBlindBit
func BenchmarkV1(startHeight, endHeight uint64, baseURL string) {
	logging.L.Info().Msgf("Starting v1 HTTP benchmark from height %d to %d", startHeight, endHeight)

	// Create client using existing networking code
	client := &networking.ClientBlindBit{BaseURL: baseURL}

	startTime := time.Now()

	// Fetch data for each height
	for height := startHeight; height <= endHeight; height++ {
		blockData, err := fetchBlockDataV1(height, client)
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to fetch block data")
			continue
		}

		logging.L.Debug().Uint64("height", height).Msg("fetched block data")
		_ = blockData // Use blockData to avoid compiler warning
	}

	duration := time.Since(startTime)
	blocksProcessed := endHeight - startHeight + 1

	logging.L.Info().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Uint64("blocks_processed", blocksProcessed).
		Dur("total_duration", duration).
		Float64("blocks_per_second", float64(blocksProcessed)/duration.Seconds()).
		Msg("v1 HTTP benchmark completed")
}

// fetchBlockDataV1 fetches block data for a single height using existing ClientBlindBit
func fetchBlockDataV1(height uint64, client *networking.ClientBlindBit) (*BlockDataV1, error) {
	var wg sync.WaitGroup
	wg.Add(3)

	errChan := make(chan error, 3)

	var filterNew, filterSpent *networking.Filter
	var tweaks [][33]byte

	// Fetch new UTXOs filter
	go func() {
		defer wg.Done()
		var err error
		filterNew, err = client.GetFilter(height, networking.NewUTXOFilterType)
		if err != nil {
			logging.L.Err(err).Msg("failed to get new utxos filter")
			errChan <- err
		}
	}()

	// Fetch spent outpoints filter
	go func() {
		defer wg.Done()
		var err error
		filterSpent, err = client.GetFilter(height, networking.SpentOutpointsFilterType)
		if err != nil {
			logging.L.Err(err).Msg("failed to get spent outpoints filter")
			errChan <- err
		}
	}()

	// Fetch tweaks
	go func() {
		defer wg.Done()
		var err error
		tweaks, err = client.GetTweaks(height, 0) // 0 = no dust limit
		if err != nil {
			logging.L.Err(err).Msg("failed to pull tweaks")
			errChan <- err
		}
	}()

	wg.Wait()

	select {
	case err := <-errChan:
		return nil, err
	default:
		// No errors
	}

	return &BlockDataV1{
		Height:      height,
		FilterNew:   filterNew,
		FilterSpent: filterSpent,
		Tweaks:      tweaks,
	}, nil
}

// BlockDataV1 represents the block data structure for v1
type BlockDataV1 struct {
	Height      uint64
	FilterNew   *networking.Filter
	FilterSpent *networking.Filter
	Tweaks      [][33]byte
}
