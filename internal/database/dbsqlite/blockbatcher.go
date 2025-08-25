package database

import (
	"context"
	"database/sql"
)

type BlockBatcher struct {
	tx       *sql.Tx
	insTxns  *sql.Stmt
	insBlkTx *sql.Stmt
	insOut   *sql.Stmt
	insIn    *sql.Stmt
}

// Begin a batch (opens tx, prepares once).
func BeginBatch(ctx context.Context, db *sql.DB) (*BlockBatcher, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Defer FK checks to COMMIT; resets automatically at COMMIT/ROLLBACK.
	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		_ = tx.Rollback()
		return nil, err
	} //  [oai_citation:4‡sqlite.org](https://sqlite.org/pragma.html?utm_source=chatgpt.com)

	insTxns, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO transactions(txid,tweak) VALUES (?,?)")
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	insBlkTx, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO block_txs(block_hash,position,txid) VALUES (?,?,?)")
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	insOut, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO outputs(txid,vout,amount) VALUES (?,?,?)")
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	insIn, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO inputs(spend_txid,idx,prev_txid,prev_vout) VALUES (?,?,?,?)")
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	return &BlockBatcher{tx, insTxns, insBlkTx, insOut, insIn}, nil
}

func (b *BlockBatcher) InsertBlock(ctx context.Context, blockHash []byte, height int64, txs []*Tx) error {
	// parents
	if _, err := b.tx.ExecContext(ctx, `INSERT OR IGNORE INTO headers(block_hash) VALUES (?)`, blockHash); err != nil {
		return err
	}
	if _, err := b.tx.ExecContext(ctx, `
		INSERT INTO chain_index(block_height, block_hash) VALUES (?,?)
		ON CONFLICT(block_height) DO UPDATE SET block_hash=excluded.block_hash`, height, blockHash); err != nil {
		return err
	}

	// transactions
	for _, t := range txs {
		if t.Tweak != nil {
			if _, err := b.insTxns.ExecContext(ctx, t.Txid, t.Tweak[:]); err != nil {
				return err
			}
		}
	}
	// block_txs
	for i, t := range txs {
		if _, err := b.insBlkTx.ExecContext(ctx, blockHash, i, t.Txid); err != nil {
			return err
		}
	}
	// outputs
	for _, t := range txs {
		for _, o := range t.Outs {
			if _, err := b.insOut.ExecContext(ctx, o.Txid, o.Vout, o.Amount); err != nil {
				return err
			}
		}
	}
	// inputs
	for _, t := range txs {
		for _, in := range t.Ins {
			if _, err := b.insIn.ExecContext(ctx, t.Txid, in.Idx, in.PrevTxid, in.PrevVout); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BlockBatcher) Commit() error {
	defer func() {
		_ = b.insTxns.Close()
		_ = b.insBlkTx.Close()
		_ = b.insOut.Close()
		_ = b.insIn.Close()
	}()
	return b.tx.Commit()
}

func (b *BlockBatcher) Rollback() error {
	defer func() {
		_ = b.insTxns.Close()
		_ = b.insBlkTx.Close()
		_ = b.insOut.Close()
		_ = b.insIn.Close()
	}()
	return b.tx.Rollback()
}
