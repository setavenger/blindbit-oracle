package dbpebble

import (
	"math"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// BuildStaticIndexing pulls the entire DB and rewrites as static indexes
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
func (s *Store) BuildStaticIndexing() error {
	blockhashTip, heightTip, err := s.GetChainTip()
	if err != nil {
		return err
	}
	blockhashStart, heightStart, err := s.FirstBlock()
	if err != nil {
		return err
	}

	logging.L.Info().Msgf("Building static indexes from %d -> %d", heightStart, heightTip)
	logging.L.Debug().
		Hex("blockhash_start", utils.ReverseBytesCopy(blockhashStart)).
		Hex("blockhash_tip", utils.ReverseBytesCopy(blockhashTip)).
		Uint32("height_start", heightStart).
		Uint32("height_tip", heightTip).
		Msg("indexing details")

	for height := heightStart; height <= heightTip; height++ {
		var blockhash []byte
		blockhash, err = s.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Err(err).Uint32("height", height).Msg("failed to blockash by height")
			return err
		}
		err = s.ReindexBlock(blockhash)
		if err != nil {
			logging.L.Err(err).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("static indexes failed")
			return err
		}
		if height%100 == 0 {
			logging.L.Info().Msgf("height %d", height)
		}
	}

	return err
}

func (s *Store) ReindexBlock(blockhash []byte) error {
	outputs, err := s.FetchOutputsAll(blockhash, math.MaxUint32)
	if err != nil {
		return err
	}

	tweaks, err := s.TweaksForBlockAll(blockhash)
	if err != nil {
		return err
	}

	outputDBValue := convertOutputsToStaticDBValue(outputs)
	tweaksDBValue := convertTweakRowToStaticDBValue(tweaks)

	err = s.finishBlockStatics(blockhash, tweaksDBValue, outputDBValue)
	if err != nil {
		return err
	}
	return err
}

func (s *Store) finishBlockStatics(
	blockhash, tweaks, outputs []byte,
) error {
	s.batchSync.Lock()
	err := attachStaticsToBatch(s.dbBatch, blockhash, tweaks, outputs)
	if err != nil {
		return err
	}
	s.batchSync.Unlock()

	s.batchCounter++
	if s.batchCounter >= 1000 {
		err = s.commitBatch()
		if err != nil {
			return err
		}
	}
	return err
}

func attachStaticsToBatch(
	batch *pebble.Batch,
	blockhash, tweaks, outputs []byte,
) error {
	if err := batch.Set(KeyTweaksStatic(blockhash), tweaks, nil); err != nil {
		return err
	}

	if err := batch.Set(KeyKUTXOsStatic(blockhash), outputs, nil); err != nil {
		return err
	}

	return nil
}

func (s *Store) writeStaticsTweaks(tweaks []database.TweakRow) error {
	return nil
}

func (s *Store) writeStaticsOutputs(outputs []*database.Output) error {
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
