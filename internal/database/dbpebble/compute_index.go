package dbpebble

import (
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

// should we encode the count of outputs?
type ComputeIndex struct {
	TxId         []byte // 32 bytes
	Height       uint32
	Tweak        []byte    // 33 bytes
	OutputsShort [][8]byte // 8 bytes each
}

func (c *ComputeIndex) SerialiseKey() []byte {
	return KeyComputeIndex(c.Height, c.TxId)
}

func (c *ComputeIndex) SerialiseData() []byte {
	totalLength := SizeTweak + len(c.OutputsShort)*8
	flattened := make([]byte, totalLength)

	// Copy tweak data
	copy(flattened[:SizeTweak], c.Tweak)

	// Copy outputs data
	for i, byteArray := range c.OutputsShort {
		copy(flattened[SizeTweak+i*8:SizeTweak+(i+1)*8], byteArray[:])
	}
	return flattened
}

func (c *ComputeIndex) DeSerialiseKey(key []byte) error {
	return nil
}

func (s *Store) BuildComputeIndexByRange(startHeight, endHeight uint32) error {
	logging.L.Info().Msgf("Building static indexes from %d -> %d", startHeight, endHeight)

	heightChan := make(chan uint32, 100) // Buffered channel for heights
	errChan := make(chan error)
	var wg sync.WaitGroup

	// Send heights to channel
	go func() {
		defer close(heightChan)
		for height := startHeight; height <= endHeight; height++ {
			heightChan <- height
		}
	}()

	// Start worker goroutines
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range heightChan {
				blockComputeIndexes, err := s.BuildComputeIndexForHeight(height)
				if err != nil {
					logging.L.Err(err).
						Uint32("height", height).
						Msg("compute indexes failed")
					errChan <- err
					return
				}
				err = s.FinishComputeIndex(height, blockComputeIndexes)
				if err != nil {
					logging.L.Err(err).
						Uint32("height", height).
						Msg("compute indexes failed")
					errChan <- err
					return
				}
				if height%100 == 0 {
					logging.L.Info().Msgf("Processed height %d", height)
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

func (s *Store) BuildComputeIndexForHeight(height uint32) ([]ComputeIndex, error) {
	blockhash, err := s.GetBlockHashByHeight(height)
	if err != nil {
		return nil, err
	}

	tweaks, err := s.TweaksForBlockAll(blockhash)
	if err != nil {
		return nil, err
	}

	computeIndexes := make([]ComputeIndex, len(tweaks))
	for i := range tweaks {
		outputs, err := s.OutputsForTx(tweaks[i].Txid[:])
		if err != nil {
			return nil, err
		}
		outputsShort := make([][8]byte, len(outputs))
		for j := range outputs {
			copy(outputsShort[j][:], outputs[j].Pubkey)
		}

		computeIndexes[i] = ComputeIndex{
			TxId:         tweaks[i].Txid[:],
			Tweak:        tweaks[i].Tweak[:],
			Height:       height,
			OutputsShort: outputsShort,
		}
	}

	return computeIndexes, nil
}

func attachComputeIndexToBatch(
	batch *pebble.Batch,
	computeIndexes []ComputeIndex,
) error {
	for _, computeIndex := range computeIndexes {
		if err := batch.Set(computeIndex.SerialiseKey(), computeIndex.SerialiseData(), nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) FinishComputeIndex(height uint32, computeIndexes []ComputeIndex) error {
	s.batchSync.Lock()
	err := attachComputeIndexToBatch(s.dbBatch, computeIndexes)
	if err != nil {
		return err
	}
	s.batchSync.Unlock()

	s.batchCounter++
	if s.batchCounter >= 200 {
		err = s.commitBatch(false)
		if err != nil {
			return err
		}
	}
	return err

}
