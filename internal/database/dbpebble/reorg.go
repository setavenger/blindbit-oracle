package dbpebble

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// Reorg handling
//
// Every row a block contributes is written by attachBlockToBatch. When the
// block at a height is replaced by a different one, replaceBlock removes the
// displaced block's rows in the same pebble batch as the new block's rows:
//
//   - KCIBlock(oldHash), so the orphan no longer counts as best chain
//     (BlockhashInDB, spent checks, the reorg walk-back)
//   - KBlockTx(oldHash, *), KTxOccur(*, oldHash), KTxidOutpoints(oldHash, *),
//     KSpentOutputsShort(oldHash)
//   - KSpend(*, *, oldHash): spends that happened only in the orphan
//   - KComputeIndex(height, *): the compute index is served by height, so
//     every row at the height is dropped and the new block writes its own
//   - KTx(txid) and KOut(txid, *) of each of the orphan's transactions that no
//     other best-chain block contains. A transaction re-confirmed elsewhere in
//     the new branch keeps them; one re-confirmed in the replacing block
//     itself has them deleted and then rewritten within this same batch.
//
// KCIHeight(height) is simply overwritten by the new block.
//
// Ordering. Deletes are computed from committed state, so replaceBlock first
// flushes synchronously and waits for background commits: every block handed
// to the store before this one is then visible, including a re-confirming
// block of the new branch applied earlier. The deletes are appended to the
// batch before the new block's Sets; pebble applies a batch's operations in
// order, so a key the new block rewrites (its own KCIHeight, a re-confirmed
// tx's KTx/KOut, compute-index rows at this height) ends up set. The batch is
// then committed synchronously, so no later batch can be committed before it
// and resurrect or re-delete rows out of order.
//
// Initial sync pulls blocks concurrently and applies them out of height order,
// but only for heights that are not indexed yet, so it never takes this path
// and keeps its asynchronous batching. The path is taken when a height that is
// already indexed receives a different hash: the continuous-sync walk-back
// after a reorg, or an explicit re-sync of a range whose blocks changed.

// replaceBlock applies block over oldHash, the block currently indexed at
// block.Height, atomically removing everything oldHash contributed.
func (s *Store) replaceBlock(oldHash []byte, block *database.DBBlock) error {
	logging.L.Warn().
		Uint32("height", block.Height).
		Hex("old_blockhash", utils.ReverseBytesCopy(oldHash)).
		Str("new_blockhash", block.Hash.String()).
		Msg("reorg: replacing indexed block")

	// Make every earlier write visible before reading what to delete.
	if err := s.FlushBatch(true); err != nil {
		return err
	}

	s.batchSync.Lock()
	err := s.attachDisconnectToBatch(s.dbBatch, block.Height, oldHash)
	if err == nil {
		err = attachBlockToBatch(s.dbBatch, block)
	}
	if err != nil {
		// The batch was empty after the flush, so it holds only this block's
		// partial operations: drop them rather than commit half a reorg.
		_ = s.dbBatch.Close()
		s.dbBatch = s.DB.NewBatch()
		s.batchCounter = 0
		s.batchSync.Unlock()
		return err
	}
	s.batchCounter++
	s.batchSync.Unlock()

	return s.commitBatch(true)
}

// attachDisconnectToBatch appends deletes for every row the block oldHash at
// height contributed. It reads committed state only.
func (s *Store) attachDisconnectToBatch(b *pebble.Batch, height uint32, oldHash []byte) error {
	if len(oldHash) != SizeHash {
		return fmt.Errorf("reorg: bad blockhash length %d", len(oldHash))
	}

	// block -> tx rows, and the txids they point to
	lb, ub := BoundsBlockTx(oldHash)
	blockTxKeys, txids, err := s.collectRange(lb, ub, true)
	if err != nil {
		return err
	}
	if err := deleteKeys(b, blockTxKeys); err != nil {
		return err
	}

	for _, txid := range txids {
		if err := b.Delete(KeyTxOccur(txid, oldHash), nil); err != nil {
			return err
		}

		// spends made by this tx in the orphan, and the accelerator row listing them
		outpoints, err := s.FetchTxidOutpoints(oldHash, txid)
		if err != nil {
			return err
		}
		for _, op := range outpoints {
			key := KeySpend(op[:SizeTxid], binary.BigEndian.Uint32(op[SizeTxid:]), oldHash)
			if err := b.Delete(key, nil); err != nil {
				return err
			}
		}
		if err := b.Delete(KeyTxidOutpoints(oldHash, txid), nil); err != nil {
			return err
		}

		// tweak and outputs, unless another best-chain block still has the tx
		elsewhere, err := s.txOnBestChainElsewhere(txid, oldHash)
		if err != nil {
			return err
		}
		if elsewhere {
			continue
		}
		if err := b.Delete(KeyTx(txid), nil); err != nil {
			return err
		}
		lb, ub := BoundsOut(txid)
		outKeys, _, err := s.collectRange(lb, ub, false)
		if err != nil {
			return err
		}
		if err := deleteKeys(b, outKeys); err != nil {
			return err
		}
	}

	// Accelerator rows are per block; delete them even if the orphan had no
	// tx rows (defensive against partially written blocks).
	lb, ub = BoundsTxidOutpoints(oldHash)
	accKeys, _, err := s.collectRange(lb, ub, false)
	if err != nil {
		return err
	}
	if err := deleteKeys(b, accKeys); err != nil {
		return err
	}
	if err := b.Delete(KeySpentOutputsShort(oldHash), nil); err != nil {
		return err
	}

	// compute index rows at this height (keyed by height, so all of them)
	lb, ub = BoundsComputeIndexOneHeight(height)
	ciKeys, _, err := s.collectRange(lb, ub, false)
	if err != nil {
		return err
	}
	if err := deleteKeys(b, ciKeys); err != nil {
		return err
	}

	return b.Delete(KeyCIBlock(oldHash), nil)
}

// PurgeOrphanedBlocks removes what blocks displaced by past reorgs left
// behind in databases indexed before reorg handling existed. Such a block
// still has its blockhash->height row although the height now maps to another
// block. For every such block its rows are deleted as in a reorg, and the
// compute index of its height, which mixed both blocks' rows, is rebuilt from
// the best-chain block. It returns the number of orphaned blocks purged.
//
// On a database without orphans it costs one scan of the blockhash->height
// rows and one point lookup per row, and writes nothing.
func (s *Store) PurgeOrphanedBlocks() (int, error) {
	if err := s.FlushBatch(true); err != nil {
		return 0, err
	}

	lb, ub := []byte{KCIBlock}, []byte{KCIBlock + 1}
	keys, vals, err := s.collectRange(lb, ub, true)
	if err != nil {
		return 0, err
	}
	orphansByHeight := make(map[uint32][][]byte)
	var heights []uint32
	for i, k := range keys {
		hash := k[1:]
		if len(hash) != SizeHash || len(vals[i]) != SizeHeight {
			return 0, fmt.Errorf("malformed blockhash->height row %x=%x", k, vals[i])
		}
		h := binary.BigEndian.Uint32(vals[i])
		atHeight, err := s.GetBlockHashByHeight(h)
		if err != nil {
			return 0, err
		}
		if bytes.Equal(atHeight, hash) {
			continue
		}
		if _, seen := orphansByHeight[h]; !seen {
			heights = append(heights, h)
		}
		orphansByHeight[h] = append(orphansByHeight[h], hash)
	}

	purged := 0
	for _, h := range heights {
		// Rebuild the compute index for h from committed state before any
		// delete: the best block's rows are not touched by disconnecting the
		// orphans (shared txs are kept), so this is exactly its index.
		computeIndexes, err := s.BuildComputeIndexForHeight(h)
		if err != nil {
			return purged, err
		}

		b := s.DB.NewBatch()
		for _, orphan := range orphansByHeight[h] {
			logging.L.Warn().
				Uint32("height", h).
				Hex("blockhash", utils.ReverseBytesCopy(orphan)).
				Msg("purging block displaced by an earlier reorg")
			if err := s.attachDisconnectToBatch(b, h, orphan); err != nil {
				_ = b.Close()
				return purged, err
			}
		}
		if err := attachComputeIndexToBatch(b, computeIndexes); err != nil {
			_ = b.Close()
			return purged, err
		}
		if err := b.Commit(pebble.Sync); err != nil {
			_ = b.Close()
			return purged, err
		}
		_ = b.Close()
		purged += len(orphansByHeight[h])
	}
	return purged, nil
}

// txOnBestChainElsewhere reports whether txid occurs in a best-chain block
// other than exclude.
func (s *Store) txOnBestChainElsewhere(txid, exclude []byte) (bool, error) {
	lb, ub := BoundsTxOccur(txid)
	keys, _, err := s.collectRange(lb, ub, false)
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		blk := k[1+SizeTxid:]
		if bytes.Equal(blk, exclude) {
			continue
		}
		// full round trip: also correct while PurgeOrphanedBlocks still has
		// stale blockhash->height rows to remove
		ok, err := s.BlockhashInDB(blk)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// collectRange returns copies of the keys (and, if withValues, the values) in
// [lb, ub) of committed state.
func (s *Store) collectRange(lb, ub []byte, withValues bool) (keys, vals [][]byte, err error) {
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, nil, err
	}
	defer it.Close()
	for ok := it.First(); ok; ok = it.Next() {
		keys = append(keys, bytes.Clone(it.Key()))
		if withValues {
			vals = append(vals, bytes.Clone(it.Value()))
		}
	}
	return keys, vals, it.Error()
}

func deleteKeys(b *pebble.Batch, keys [][]byte) error {
	for _, k := range keys {
		if err := b.Delete(k, nil); err != nil {
			return err
		}
	}
	return nil
}
