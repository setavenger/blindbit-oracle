package dbpebble

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
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

	logging.L.Info().Msgf("Building static indexes from %d -> %d", heightStart, heightTip)
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

	// Start worker goroutines
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for blockhash := range blockhashChan {
				count := atomic.AddInt64(&blockCounter, 1)
				err := s.ReindexBlock(blockhash, force)
				if err != nil {
					logging.L.Err(err).
						Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
						Msg("static indexes failed")
					errChan <- err
					return
				}
				if count%100 == 0 {
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
					logging.L.Info().Msgf("Processed %d blocks, height %d", count, height)
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

func (s *Store) ReindexBlock(blockhash []byte, force bool) error {
	// make point check to see if we need to rebuild the static data
	if !force {
		existsTweaks, err := s.KeyExistsStaticTweaks(blockhash)
		if err != nil {
			logging.L.Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to check if tweaks exist")
			return err
		}
		existsOutputs, err := s.KeyExistsStaticOutputs(blockhash)
		if err != nil {
			logging.L.Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to check if outputs exist")
			return err
		}
		existsUnspentFilter, err := s.KeyExistsStaticTaprootUnspentFilter(blockhash)
		if err != nil {
			logging.L.Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to check if unspent filter exists")
			return err
		}
		if existsTweaks && existsOutputs && existsUnspentFilter {
			return nil
		}
	}
	outputs, err := s.FetchOutputsAll(blockhash, math.MaxUint32)
	if err != nil {
		return err
	}

	tweaks, err := s.TweaksForBlockAll(blockhash)
	if err != nil {
		return err
	}

	// can be integrated with filters.
	// Just add another copy of the pubkey during serialisation into the filter
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
	return err
}

func (s *Store) finishBlockStatics(
	blockhash, tweaks, outputs, unspentFilter []byte,
) error {
	s.batchSync.Lock()
	err := attachStaticsToBatch(s.dbBatch, blockhash, tweaks, outputs, unspentFilter)
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
	return indexer.BuildTaprootPubkeyFilter(blockhash, taprootOutputs)
}
