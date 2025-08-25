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

-- Block -> tx list (ordered). Contains ALL txids (not just ones in `transactions`)
CREATE TABLE IF NOT EXISTS block_txs (
  block_hash BLOB NOT NULL REFERENCES headers(block_hash),
  position   INTEGER NOT NULL,                -- 0 = coinbase, but is omitted
  txid       BLOB NOT NULL,                   -- may or may not exist in `transactions`

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

