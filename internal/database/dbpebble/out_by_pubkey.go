package dbpebble

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
)

const defaultOutByPubkeyBackfillChunkSize = 1_000_000

type outByPubkeyBackfillEntry struct {
	key   [1 + SizePubKey + SizeTxid + SizeVout]byte
	value [SizeAmt]byte
}

// outByPubkeyBackfillEntryFromOut reformats a KOut row
// (key [txid][vout], value [amount][pubkey]) into a KOutByPubkey entry
// (key [pubkey][txid][vout], value [amount]).
func outByPubkeyBackfillEntryFromOut(key, value []byte) (outByPubkeyBackfillEntry, error) {
	var entry outByPubkeyBackfillEntry
	if len(key) != 1+SizeTxid+SizeVout {
		return entry, fmt.Errorf("bad out key length: %d", len(key))
	}
	if len(value) != SizeAmt+SizePubKey {
		return entry, fmt.Errorf("bad out value length: %d", len(value))
	}

	entry.key[0] = KOutByPubkey
	copy(entry.key[1:1+SizePubKey], value[SizeAmt:SizeAmt+SizePubKey])
	copy(entry.key[1+SizePubKey:1+SizePubKey+SizeTxid], key[1:1+SizeTxid])
	copy(entry.key[1+SizePubKey+SizeTxid:], key[1+SizeTxid:1+SizeTxid+SizeVout])
	copy(entry.value[:], value[:SizeAmt])

	return entry, nil
}

// BuildOutByPubkeyIndex backfills KOutByPubkey from existing KOut data.
//
// It performs one continuous scan over KOut, reformats rows into
// [pubkey][txid][vout] order, sorts bounded chunks in memory, and writes each
// chunk in a batch. It does not perform pubkey-by-pubkey lookups.
func (s *Store) BuildOutByPubkeyIndex(ctx context.Context, chunkSize int) (uint64, error) {
	if chunkSize <= 0 {
		chunkSize = defaultOutByPubkeyBackfillChunkSize
	}

	it, err := s.DB.NewIter(&pebble.IterOptions{
		LowerBound: []byte{KOut},
		UpperBound: []byte{KOut + 1},
	})
	if err != nil {
		return 0, err
	}
	defer it.Close()

	entries := make([]outByPubkeyBackfillEntry, 0, chunkSize)
	var total uint64

	flush := func() error {
		if len(entries) == 0 {
			return nil
		}

		sort.Slice(entries, func(i, j int) bool {
			return bytes.Compare(entries[i].key[:], entries[j].key[:]) < 0
		})

		batch := s.DB.NewBatch()
		defer batch.Close()

		for i := range entries {
			if err := batch.Set(entries[i].key[:], entries[i].value[:], nil); err != nil {
				return err
			}
		}

		if err := batch.Commit(pebble.NoSync); err != nil {
			return err
		}

		total += uint64(len(entries))
		logging.L.Info().
			Uint64("total_rows", total).
			Int("chunk_rows", len(entries)).
			Msg("built out-by-pubkey index chunk")
		entries = entries[:0]
		return nil
	}

	for ok := it.First(); ok; ok = it.Next() {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		entry, err := outByPubkeyBackfillEntryFromOut(it.Key(), it.Value())
		if err != nil {
			return total, err
		}

		entries = append(entries, entry)
		if len(entries) >= chunkSize {
			if err := flush(); err != nil {
				return total, err
			}
		}
	}

	if err := it.Error(); err != nil {
		return total, err
	}
	if err := flush(); err != nil {
		return total, err
	}

	logging.L.Info().Uint64("total_rows", total).Msg("finished building out-by-pubkey index")
	return total, nil
}
