package database

import (
	"context"
	"database/sql"
)

func DropIndexesForIBD(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		"DROP INDEX IF EXISTS ux_block_txs_block_txid",
		"DROP INDEX IF EXISTS ix_block_txs_txid",
		"DROP INDEX IF EXISTS ix_inputs_prevout",
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func CreateIndexesAfterIBD(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_block_txs_block_txid ON block_txs(block_hash, txid)",
		"CREATE INDEX IF NOT EXISTS ix_block_txs_txid ON block_txs(txid)",
		"CREATE INDEX IF NOT EXISTS ix_inputs_prevout ON inputs(prev_txid, prev_vout)",
		"PRAGMA optimize",
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
