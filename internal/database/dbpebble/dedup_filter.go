package dbpebble

import (
	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// pubkeyFundedMoreThanOnce reports whether the given x-only pubkey has at least
// two funding rows in the KOutByPubkey accelerator index. It scans the bounded
// pubkey range and stops after the second row, so it never scans the whole DB.
func (s *Store) pubkeyFundedMoreThanOnce(pubkey []byte) (bool, error) {
	lb, ub := BoundsOutByPubkey(pubkey)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return false, err
	}
	defer it.Close()

	count := 0
	for ok := it.First(); ok; ok = it.Next() {
		count++
		if count >= 2 {
			break
		}
	}
	if err := it.Error(); err != nil {
		return false, err
	}
	return count >= 2, nil
}

// FetchComputeIndexDedupTaproot behaves like FetchComputeIndex but excludes
// taproot outputs whose x-only pubkey was funded more than once across the
// indexed chain history (looked up via the KOutByPubkey accelerator index).
// Transactions left with no eligible outputs are dropped entirely. It also
// returns per-block stats for benchmark logging.
//
// The compute-index short pubkeys are stored in vout order, identical to the
// order OutputsForTx returns, so the j-th 8-byte short corresponds to the j-th
// full output pubkey and the two can be aligned by index.
func (s *Store) FetchComputeIndexDedupTaproot(
	height uint32,
) ([]*pb.ComputeIndexTxItem, database.DedupFilterStats, error) {
	var stats database.DedupFilterStats

	lb, ub := BoundsComputeIndexOneHeight(height)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, stats, err
	}
	defer it.Close()

	var out []*pb.ComputeIndexTxItem
	for ok := it.First(); ok; ok = it.Next() {
		stats.TxsTotal++

		// copy iterator-owned bytes; they are only valid until the next Next()
		keySrc := it.Key()
		valSrc := it.Value()

		// internal (non-reversed) txid, as used by KOut / KOutByPubkey keys
		txid := make([]byte, SizeTxid)
		copy(txid, keySrc[1+SizeHeight:])

		val := make([]byte, len(valSrc))
		copy(val, valSrc)

		tweak := val[:SizeTweak]
		shorts := val[SizeTweak:]
		n := len(shorts) / 8
		stats.TotalOutputs += n

		outputs, err := s.OutputsForTx(txid)
		if err != nil {
			return nil, stats, err
		}

		// Defensive: if short-count and full-output-count disagree we cannot
		// align them safely, so serve the row unfiltered rather than corrupt it.
		if len(outputs) != n {
			logging.L.Warn().
				Uint32("height", height).
				Int("shorts", n).
				Int("outputs", len(outputs)).
				Msg("dedup filter: output count mismatch, serving tx unfiltered")
			stats.SentOutputs += n
			out = append(out, &pb.ComputeIndexTxItem{
				Txid:         utils.ReverseBytesCopy(txid),
				Tweak:        tweak,
				OutputsShort: shorts,
			})
			continue
		}

		filtered := make([]byte, 0, len(shorts))
		kept := 0
		for j := 0; j < n; j++ {
			dup, err := s.pubkeyFundedMoreThanOnce(outputs[j].Pubkey)
			if err != nil {
				return nil, stats, err
			}
			if dup {
				continue
			}
			filtered = append(filtered, shorts[j*8:(j+1)*8]...)
			kept++
		}

		stats.SentOutputs += kept
		stats.OmittedOutputs += n - kept

		if kept == 0 {
			stats.TxsDropped++
			continue
		}

		out = append(out, &pb.ComputeIndexTxItem{
			Txid:         utils.ReverseBytesCopy(txid),
			Tweak:        tweak,
			OutputsShort: filtered,
		})
	}
	if err := it.Error(); err != nil {
		return nil, stats, err
	}

	return out, stats, nil
}
