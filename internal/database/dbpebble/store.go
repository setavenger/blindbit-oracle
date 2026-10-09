package dbpebble

import (
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

var _ database.DB = (*Store)(nil)

type Store struct {
	DB           *pebble.DB
	dbBatch      *pebble.Batch
	batchCounter int
	batchSync    *sync.Mutex
	batchSize    int
	// maximum number of batches to keep in memory
	maxPendingCommits int64

	// Guard fields for safe closing
	pendingCommits int64          // atomic counter for pending background commits
	closed         int32          // atomic flag to indicate if store is closed
	closeWaitGroup sync.WaitGroup // wait group for pending commits
}

func NewStore(db *pebble.DB) *Store {
	return &Store{
		DB:                db,
		dbBatch:           db.NewBatch(),
		batchCounter:      0,
		maxPendingCommits: 10,
		batchSync:         new(sync.Mutex),
		batchSize:         200,
	}
}

func (s *Store) BatchSize() int {
	s.batchSync.Lock()
	defer s.batchSync.Unlock()
	return s.batchCounter
}

// rotateLocked rotate swaps s.dbBatch to a fresh one under the lock
// and returns the old batch to commit outside.
func (s *Store) rotateLocked() (old *pebble.Batch) {
	old = s.dbBatch
	s.dbBatch = s.DB.NewBatch()
	s.batchCounter = 0
	return
}

func (s *Store) collectAndWrite(block *database.DBBlock) error {
	// var toCommit *pebble.Batch

	s.batchSync.Lock()
	if s.dbBatch == nil {
		s.dbBatch = s.DB.NewBatch()
	}
	s.batchCounter++
	shouldCommit := s.batchCounter > s.batchSize
	s.batchSync.Unlock()

	if shouldCommit {
		// rotates batch and commits the old one in the background
		if err := s.commitBatch(false); err != nil {
			logging.L.Err(err).Msg("failed to commit batch")
			return err
		}
	}

	// write to the (possibly new) active batch
	if err := s.attachBlockToBatch(block); err != nil {
		logging.L.Err(err).Msg("failed to attach to batch")
		return err
	}

	return nil
}

func (s *Store) attachBlockToBatch(block *database.DBBlock) error {
	s.batchSync.Lock()
	defer s.batchSync.Unlock()
	return attachBlockToBatch(s.dbBatch, block)
}

func (s *Store) FlushBatch(sync bool) error {
	s.batchSync.Lock()
	counter := s.batchCounter
	s.batchSync.Unlock()

	if counter == 0 {
		return nil
	}
	logging.L.Info().
		Int("batch_counter", counter).
		Bool("sync", sync).
		Msg("flushing batch")

	return s.commitBatch(sync)
}

// commitBatch commits the batch and rotates the old batch
// if sync is true, the batch is committed synchronously
// false is for background commits
func (s *Store) commitBatch(sync bool) error {
	// Check if store is already closed
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil // Store is closed, don't commit
	}

	if atomic.LoadInt64(&s.pendingCommits) >= int64(s.maxPendingCommits) {
		// wait for pending commits to complete
		logging.L.Info().
			Int64("pending_commits", atomic.LoadInt64(&s.pendingCommits)).
			Int64("max_pending_commits", int64(s.maxPendingCommits)).
			Msg("waiting for pending commits")
		s.WaitForPendingCommits()
	}

	s.batchSync.Lock()

	// rotate out the old bath and commit the old one subsequently
	oldBatch := s.rotateLocked()
	s.batchSync.Unlock()

	closeOldBatch := func() error {
		defer oldBatch.Close()
		// this might need a max commit semaphore style lock or something
		err := oldBatch.Commit(pebble.NoSync)
		if err != nil {
			logging.L.Panic().Err(err).Msg("failed to write Batch")
			return err
		}
		return nil
	}

	if sync {
		return closeOldBatch()
	} else {
		// Track pending background commit
		atomic.AddInt64(&s.pendingCommits, 1)
		s.closeWaitGroup.Add(1)

		go func() {
			defer func() {
				atomic.AddInt64(&s.pendingCommits, -1)
				s.closeWaitGroup.Done()
			}()

			err := closeOldBatch()
			if err != nil {
				logging.L.Err(err).Msg("failed to write Batch")
			}
		}()
		return nil
	}
}

func attachBlockToBatch(batch *pebble.Batch, block *database.DBBlock) error {
	blockHash := block.Hash[:]
	txs := block.Txs
	height := block.Height

	b := batch

	// chain index
	if err := b.Set(KeyCIHeight(height), blockHash, nil); err != nil {
		logging.L.Err(err).Msg("insert failed")
		return err
	}
	hb := make([]byte, SizeHeight)
	be32(height, hb)
	if err := b.Set(KeyCIBlock(blockHash), hb, nil); err != nil {
		logging.L.Err(err).Msg("insert failed")
		return err
	}

	// Collect spent outputs short (first 8 bytes of x-only pubkeys)
	var spentOutputsShort [][8]byte // First 8 bytes of x-only pubkeys

	// Collect txid to outpoints mappings
	txidOutpointsMap := make(map[[32]byte][][36]byte) // txid -> outpoints array

	// block → txs + transaction records
	for i := range txs {
		t := txs[i]
		if t == nil {
			// we skip nil txs here to so that we have exact positions.
			// todo: should we keep it like this?
			continue
		}
		// bt
		if err := b.Set(KeyBlockTx(blockHash, uint32(i)), t.Txid, nil); err != nil {
			logging.L.Err(err).Msg("insert failed")
			return err
		}
		// tb (optional, but handy for reorg/tools)
		if err := b.Set(KeyTxOccur(t.Txid, blockHash), nil, nil); err != nil {
			logging.L.Err(err).Msg("insert failed")
			return err
		}

		// we keep them here to not do it twice for compute index
		var txSpentsShort [][8]byte

		// spend events
		for _, in := range t.Ins {
			val, err := ValSpend(in.Pubkey) // or nil for keys-only
			if err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}
			err = b.Set(KeySpend(in.PrevTxid, in.PrevVout, blockHash), val, nil)
			if err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}

			// Collect first 8 bytes of x-only pubkey for spent outputs
			var outputShort [8]byte
			copy(outputShort[:], in.Pubkey[:8])
			txSpentsShort = append(txSpentsShort, outputShort)
		}

		// tx tweak
		if t.Tweak != nil {
			// Data only stored if a valid tweak exists
			// - tweaks
			// - outputs (new utxos; spent is always relevant)
			// - compute index

			val, err := ValTxTweak(t.Tweak[:])
			if err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}
			if err := b.Set(KeyTx(t.Txid), val, nil); err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}

			// outputs
			var newOutsShort [][8]byte
			for _, o := range t.Outs {
				val, err := ValOut(o.Amount, o.Pubkey)
				if err != nil {
					logging.L.Err(err).Any("output", o).Msg("insert failed")
					return err
				}
				if err := b.Set(KeyOut(o.Txid, o.Vout), val, nil); err != nil {
					logging.L.Err(err).Any("output", o).Msg("insert failed")
					return err
				}
				var newOut [8]byte
				copy(newOut[:], o.Pubkey[:8])
				newOutsShort = append(newOutsShort, newOut)
				// optional accelerator outv:<txid>:<amountBE>:<voutBE> -> "" could be added later
			}

			computeIndex := ComputeIndex{
				TxId:         t.Txid,
				Height:       height,
				Tweak:        t.Tweak[:],
				OutputsShort: newOutsShort,
			}
			if err := b.Set(computeIndex.SerialiseKey(), computeIndex.SerialiseData(), nil); err != nil {
				return err
			}
		}

		// Collect outpoints for txid-outpoints mapping
		var txOutpoints [][36]byte
		for _, in := range t.Ins {
			var outpoint [36]byte
			copy(outpoint[:32], in.PrevTxid)
			be32(in.PrevVout, outpoint[32:])
			txOutpoints = append(txOutpoints, outpoint)
		}
		if len(txOutpoints) > 0 {
			txidOutpointsMap[[32]byte(t.Txid)] = txOutpoints
		}

		spentOutputsShort = append(spentOutputsShort, txSpentsShort...)
	}

	// Store spent outputs short (first 8 bytes of x-only pubkeys)
	if len(spentOutputsShort) > 0 {
		spentOutputsValue := make([]byte, 8*len(spentOutputsShort))
		for i, outputShort := range spentOutputsShort {
			copy(spentOutputsValue[i*8:(i+1)*8], outputShort[:])
		}
		if err := b.Set(KeySpentOutputsShort(blockHash), spentOutputsValue, nil); err != nil {
			logging.L.Err(err).Msg("insert spent outputs short failed")
			return err
		}
	} else {
		// Store empty array for blocks with no spent outputs
		logging.L.Warn().
			Uint32("height", height).
			Hex("block_hash", utils.ReverseBytesCopy(blockHash)).
			Msg("no spent outputs for block")
		if err := b.Set(KeySpentOutputsShort(blockHash), []byte{}, nil); err != nil {
			logging.L.Err(err).Msg("insert spent outputs short failed")
			return err
		}
	}

	// Store txid to outpoints mappings
	for txid, outpoints := range txidOutpointsMap {
		val, err := ValTxidOutpoints(outpoints)
		if err != nil {
			logging.L.Err(err).Hex("txid", txid[:]).Msg("failed to encode txid outpoints")
			return err
		}
		if err := b.Set(KeyTxidOutpoints(blockHash, txid[:]), val, nil); err != nil {
			logging.L.Err(err).Hex("txid", txid[:]).Msg("insert txid outpoints failed")
			return err
		}
	}

	return nil
}

func (s *Store) ApplyBlock(block *database.DBBlock) error {
	return s.collectAndWrite(block)
}

// WaitForPendingCommits waits for all pending background commits to complete
func (s *Store) WaitForPendingCommits() {
	logging.L.Info().
		Int64("pending_commits", atomic.LoadInt64(&s.pendingCommits)).
		Msg("waiting for pending commits")
	s.closeWaitGroup.Wait()
	logging.L.Info().Msg("all pending commits completed")
}

// Close safely closes the store by waiting for all pending commits before closing the database
func (s *Store) Close() error {
	// Mark store as closed to prevent new commits
	atomic.StoreInt32(&s.closed, 1)

	// Wait for all pending background commits to complete
	s.WaitForPendingCommits()

	// Flush any remaining batch synchronously
	if err := s.FlushBatch(true); err != nil {
		logging.L.Err(err).Msg("failed to flush final batch")
		return err
	}

	// Close the underlying database
	if err := s.DB.Close(); err != nil {
		logging.L.Err(err).Msg("failed to close database")
		return err
	}

	logging.L.Info().Msg("store closed successfully")
	return nil
}
