package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"time"

	"github.com/setavenger/blindbit-oracle/internal/config"
	_ "modernc.org/sqlite" // driver
)

// var schemaSQL string

func OpenDB(path string) (*sql.DB, error) {
	// DSN with PRAGMAs: WAL, NORMAL sync, FK on, 5s busy timeout
	// dsn := "file:" + filepath.Join(config.BaseDirectory, "db") +
	// 	"?_pragma=foreign_keys(ON)" +
	// 	"&_pragma=journal_mode(WAL)" +
	// 	"&_pragma=synchronous(NORMAL)" +
	// 	"&_pragma=busy_timeout(5000)"

	dsn := "file:" + filepath.Join(config.BaseDirectory, "data", "db") +
		"?_txlock=immediate" + // BEGIN IMMEDIATE-style txns
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(OFF)" +
		"&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Keep the pool simple: SQLite shines with a small pool. Start with 1.
	// You can raise for read-heavy workloads, but 1 avoids SQLITE_BUSY during IBD.
	db.SetMaxOpenConns(1) // see discussion on single-connection pools :contentReference[oaicite:7]{index=7}
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create schema
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

const schemaSQL = `
PRAGMA foreign_keys = ON;

-- Headers: one row per block known (best-chain or side-chain)
CREATE TABLE IF NOT EXISTS headers (
  block_hash BLOB PRIMARY KEY
) STRICT, WITHOUT ROWID;

-- Best-chain map: height -> block_hash
CREATE TABLE IF NOT EXISTS chain_index (
  block_height INTEGER PRIMARY KEY,
  block_hash   BLOB NOT NULL REFERENCES headers(block_hash)
) STRICT, WITHOUT ROWID;

-- Only transactions you care about (Taproot/SP-relevant) – holds the tweak
CREATE TABLE IF NOT EXISTS transactions (
  txid  BLOB PRIMARY KEY,
  tweak BLOB
) STRICT, WITHOUT ROWID;

-- Block -> tx list (ordered). Contains ALL txids (not just ones in 'transactions')
CREATE TABLE IF NOT EXISTS block_txs (
  block_hash BLOB NOT NULL REFERENCES headers(block_hash),
  position   INTEGER NOT NULL,                -- 0 = coinbase, but is omitted
  txid       BLOB NOT NULL,                   -- may or may not exist in 'transactions'

  PRIMARY KEY (block_hash, position)
) STRICT, WITHOUT ROWID;

-- Helpful unique to target a specific occurrence by (block, txid)
CREATE UNIQUE INDEX IF NOT EXISTS ux_block_txs_block_txid ON block_txs(block_hash, txid);
CREATE INDEX IF NOT EXISTS ix_block_txs_txid ON block_txs(txid);

-- Outputs (Taproot outputs you index)
CREATE TABLE IF NOT EXISTS outputs (
  txid   BLOB    NOT NULL REFERENCES transactions(txid),
  vout   INTEGER NOT NULL,
  amount INTEGER NOT NULL, -- sats

  PRIMARY KEY (txid, vout)
) STRICT, WITHOUT ROWID;

-- Inputs (spend events for those outputs). No FK to transactions (spender may not be SP)
CREATE TABLE IF NOT EXISTS inputs (
  spend_txid BLOB    NOT NULL,
  idx        INTEGER NOT NULL,
  prev_txid  BLOB    NOT NULL,
  prev_vout  INTEGER NOT NULL,

  PRIMARY KEY (spend_txid, idx)
) STRICT, WITHOUT ROWID;

-- Fast “outpoint → spender(s)” lookup
CREATE INDEX IF NOT EXISTS ix_inputs_prevout ON inputs(prev_txid, prev_vout);
`
