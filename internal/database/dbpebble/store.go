package dbpebble

import (
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

type Out struct {
	Txid   []byte
	Vout   uint32
	Amount uint64
	Pubkey []byte // 32B x-only
}

type In struct {
	SpendTxid []byte // not needed for core queries; keep if you add a spender→prevout index
	Idx       uint32
	PrevTxid  []byte
	PrevVout  uint32
	Pubkey    []byte // optional 32B (taproot key path spend x-only)
}

type Tx struct {
	Txid  []byte
	Tweak *[33]byte // 33B or nil
	Outs  []*Out
	Ins   []*In
}

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

func (s *Store) collectAndWrite(block *database.DBBlock) error {
	s.batchSync.Lock()
	if s.dbBatch == nil {
		s.dbBatch = s.DB.NewBatch()
	}

	s.batchCounter++
	if s.batchCounter > s.batchSize {
		writeBatchStart := time.Now()
		err := s.dbBatch.Commit(pebble.NoSync)
		if err != nil {
			logging.L.Err(err).Msg("failed to write Batch")
			return err
		}
		logging.L.Warn().Dur("write_batch_duration", time.Since(writeBatchStart)).Msg("batch_write_bench")
		err = s.dbBatch.Close()
		if err != nil {
			logging.L.Err(err).Msg("failed to close db batch")
			return err
		}
		// s.dbBatch = nil
		s.dbBatch = s.DB.NewBatch()
		s.batchCounter = 0
	}
	err := attachBlcokToBatch(s.dbBatch, block)
	if err != nil {
		logging.L.Err(err).Msg("failed to attach to batch")
		return err
	}

	s.batchSync.Unlock()
	return nil
}

func attachBlcokToBatch(batch *pebble.Batch, block *database.DBBlock) error {
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
	for i, t := range txs {
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
			if err := b.Set(KeySpend(in.PrevTxid, in.PrevVout, blockHash), val, nil); err != nil {
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

// func (s *Store) ApplyBlock(block *database.DBBlock) error {
// 	insertStart := time.Now()
// 	defer func() {
// 		logging.L.Trace().Dur("insert_time", time.Since(insertStart)).Msg("db insertion done")
// 	}()
//
// 	b := s.DB.NewBatch()
//
// 	err := attachBlcokToBatch(b, block)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed to build batch data")
// 	}
// 	wopts := pebble.NoSync
// 	// if sync {
// 	// 	wopts = pebble.Sync
// 	// }
// 	err = b.Commit(wopts)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed to commit db tx")
// 		return err
// 	}
// 	return err
// }
