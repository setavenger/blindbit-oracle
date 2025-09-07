package dbpebble

import (
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

type Store struct {
	DB           *pebble.DB
	dbBatch      *pebble.Batch
	batchCounter int
	batchSync    *sync.Mutex
	batchSize    int
}

func NewStore(db *pebble.DB) *Store {
	return &Store{
		DB:           db,
		dbBatch:      db.NewBatch(),
		batchCounter: 0,
		batchSync:    new(sync.Mutex),
		batchSize:    200,
	}
}

func (s *Store) BatchSize() int {
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
	s.batchSync.Unlock()

	s.batchCounter++
	if s.batchCounter > s.batchSize {
		// rotates batch and commits the old one in the background
		if err := s.commitBatch(false); err != nil {
			s.batchSync.Unlock()
			logging.L.Err(err).Msg("failed to commit batch")
			return err
		}
	}

	// write to the (possibly new) active batch
	if err := s.attachBlockToBatch(block); err != nil {
		s.batchSync.Unlock()
		logging.L.Err(err).Msg("failed to attach to batch")
		return err
	}

	// // Commit outside the lock to avoid blocking writers.
	// if toCommit != nil {
	// 	writeBatchStart := time.Now()
	// 	if err := toCommit.Commit(pebble.NoSync); err != nil {
	// 		logging.L.Err(err).Msg("failed to write Batch")
	// 		return err
	// 	}
	// 	if err := toCommit.Close(); err != nil {
	// 		logging.L.Err(err).Msg("failed to close db batch")
	// 		return err
	// 	}
	// 	logging.L.Debug().
	// 		Dur("write_batch_duration", time.Since(writeBatchStart)).
	// 		Msg("batch_write_bench")
	// }
	return nil
}

func (s *Store) attachBlockToBatch(block *database.DBBlock) error {
	s.batchSync.Lock()
	defer s.batchSync.Unlock()
	return attachBlockToBatch(s.dbBatch, block)
}

func (s *Store) FlushBatch(sync bool) error {
	if s.batchCounter == 0 {
		return nil
	}
	logging.L.Info().Int("batch_counter", s.batchCounter).Bool("sync", sync).Msg("flushing batch")

	return s.commitBatch(sync)
}

func (s *Store) commitBatch(sync bool) error {
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
		go func() {
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

		// tx tweak
		if t.Tweak != nil {
			val, err := ValTxTweak(t.Tweak[:])
			if err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}
			if err := b.Set(KeyTx(t.Txid), val, nil); err != nil {
				logging.L.Err(err).Msg("insert failed")
				return err
			}
		}

		// outputs
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
			// optional accelerator outv:<txid>:<amountBE>:<voutBE> -> "" could be added later
		}

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
		}
	}

	return nil
}

func (s *Store) ApplyBlock(block *database.DBBlock) error {
	return s.collectAndWrite(block)
}
