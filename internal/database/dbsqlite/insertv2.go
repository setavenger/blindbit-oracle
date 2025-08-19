package database

import (
	"context"
	"database/sql"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
)

func InsertBlock(ctx context.Context, db *sql.DB, blockHash []byte, height int64, txs []*Tx) (err error) {
	// Start an explicit immediate transaction (driver option also set via DSN)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Defer FK checks to COMMIT (per-transaction switch).
	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		return err
	}

	// Prepare once.
	insTxns, _ := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO transactions(txid,tweak) VALUES (?,?)")
	insBlkTx, _ := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO block_txs(block_hash,position,txid) VALUES (?,?,?)")
	insOut, _ := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO outputs(txid,vout,amount) VALUES (?,?,?)")
	insIn, _ := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO inputs(spend_txid,idx,prev_txid,prev_vout) VALUES (?,?,?,?)")
	defer insTxns.Close()
	defer insBlkTx.Close()
	defer insOut.Close()
	defer insIn.Close()

	// 1) parents
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO headers(block_hash) VALUES (?)`, blockHash); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
  INSERT INTO chain_index(block_height, block_hash) VALUES (?,?)
  ON CONFLICT(block_height) DO UPDATE SET block_hash=excluded.block_hash`, height, blockHash); err != nil {
		return err
	}

	// 2) transactions with tweaks
	for _, t := range txs {
		if t.Tweak != nil {
			if _, err = insTxns.ExecContext(ctx, t.Txid, t.Tweak[:]); err != nil {
				return err
			}
		}
	}
	// 3) block_txs
	for i, t := range txs {
		if _, err = insBlkTx.ExecContext(ctx, blockHash, i, t.Txid); err != nil {
			return err
		}
	}
	// 4) outputs
	for _, t := range txs {
		for _, o := range t.Outs {
			if _, err = insOut.ExecContext(ctx, o.Txid, o.Vout, o.Amount); err != nil {
				return err
			}
		}
	}
	// 5) inputs
	for _, t := range txs {
		for _, in := range t.Ins {
			if _, err = insIn.ExecContext(ctx, t.Txid, in.Idx, in.PrevTxid, in.PrevVout); err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	// Commit
	if err != nil {
		logging.L.Err(err).Hex("blockhash", utils.ReverseBytes(blockHash)).Int64("height", height).
			Msg("failed to commit db transaction")
		return err
	}
	logging.L.Debug().Hex("blockhash", utils.ReverseBytes(blockHash)).Int64("height", height).
		Msg("successful db commit")
	return nil
}
