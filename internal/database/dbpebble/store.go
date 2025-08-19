package dbpebble

import (
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

type Store struct{ DB *pebble.DB }

func (s *Store) ApplyBlock(block *database.DBBlock) error {
	blockHash := block.Hash[:]
	txs := block.Txs
	height := block.Height

	b := s.DB.NewBatch()
	defer b.Close()

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

	wopts := pebble.NoSync
	// if sync {
	// 	wopts = pebble.Sync
	// }
	err := b.Commit(wopts)
	if err != nil {
		logging.L.Err(err).Msg("failed to commit db tx")
		return err
	}
	return err
}
