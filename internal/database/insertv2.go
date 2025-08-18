package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
)

const maxParams = 999 // safe default; many builds now allow 32766, but 999 works everywhere. :contentReference[oaicite:2]{index=2}

func InsertBlock(ctx context.Context, db *sql.DB, blockHash []byte, height int64, txs []*Tx) (err error) {
	// Start an explicit immediate transaction (driver option also set via DSN)
	if _, err = db.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_, _ = db.ExecContext(ctx, "ROLLBACK")
		}
	}()

	// Parents first (satisfy FKs)
	if _, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO headers(block_hash) VALUES (?)`, blockHash); err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO chain_index(block_height, block_hash) VALUES (?,?)
		ON CONFLICT(block_height) DO UPDATE SET block_hash=excluded.block_hash`,
		height, blockHash); err != nil {
		return err
	}

	// 1) transactions (only those with tweaks) — few rows, simple loop OK
	{
		var b strings.Builder
		args := make([]any, 0, len(txs)*2)
		count := 0
		for _, t := range txs {
			if t.Tweak == nil {
				continue
			}
			if count == 0 {
				b.WriteString("INSERT OR IGNORE INTO transactions(txid,tweak) VALUES ")
			} else {
				b.WriteByte(',')
			}
			b.WriteString("(?,?)")
			args = append(args, t.Txid, t.Tweak[:])
			count++
		}
		if count > 0 {
			if _, err = db.ExecContext(ctx, b.String(), args...); err != nil {
				return err
			}
		}
	}

	// 2) block_txs (all txs) — multi-row insert with chunking
	{
		const cols = 3
		maxRows := maxParams / cols
		start := 0
		for start < len(txs) {
			end := start + maxRows
			if end > len(txs) {
				end = len(txs)
			}
			var b strings.Builder
			args := make([]any, 0, (end-start)*cols)
			b.WriteString("INSERT OR IGNORE INTO block_txs(block_hash,position,txid) VALUES ")
			for i := start; i < end; i++ {
				if i > start {
					b.WriteByte(',')
				}
				b.WriteString("(?,?,?)")
				args = append(args, blockHash, i, txs[i].Txid)
			}
			if _, err = db.ExecContext(ctx, b.String(), args...); err != nil {
				return err
			}
			start = end
		}
	}

	// 3) outputs — only Taproot outs you track
	{
		const cols = 3
		maxRows := maxParams / cols
		type row struct {
			txid   []byte
			vout   uint32
			amount int64
		}
		var rows []row
		for _, t := range txs {
			for _, o := range t.Outs {
				rows = append(rows, row{o.Txid, o.Vout, o.Amount})
			}
		}
		for start := 0; start < len(rows); {
			end := start + maxRows
			if end > len(rows) {
				end = len(rows)
			}
			var b strings.Builder
			args := make([]any, 0, (end-start)*cols)
			b.WriteString("INSERT OR IGNORE INTO outputs(txid,vout,amount) VALUES ")
			for i := start; i < end; i++ {
				if i > start {
					b.WriteByte(',')
				}
				b.WriteString("(?,?,?)")
				r := rows[i]
				args = append(args, r.txid, r.vout, r.amount)
			}
			if _, err = db.ExecContext(ctx, b.String(), args...); err != nil {
				logging.L.Err(err).Msg("outputs batch insert failed")
				return err
			}
			start = end
		}
	}

	// 4) inputs — spend events (spenders of your tracked outs)
	{
		const cols = 4
		maxRows := maxParams / cols
		type row struct {
			stx   []byte
			idx   uint32
			ptx   []byte
			pvout uint32
		}
		var rows []row
		for _, t := range txs {
			for _, in := range t.Ins {
				rows = append(rows, row{t.Txid, in.Idx, in.PrevTxid, in.PrevVout})
			}
		}
		for start := 0; start < len(rows); {
			end := start + maxRows
			if end > len(rows) {
				end = len(rows)
			}
			var b strings.Builder
			args := make([]any, 0, (end-start)*cols)
			b.WriteString("INSERT OR IGNORE INTO inputs(spend_txid,idx,prev_txid,prev_vout) VALUES ")
			for i := start; i < end; i++ {
				if i > start {
					b.WriteByte(',')
				}
				b.WriteString("(?,?,?,?)")
				r := rows[i]
				args = append(args, r.stx, r.idx, r.ptx, r.pvout)
			}
			if len(args) > 0 {
				if _, err = db.ExecContext(ctx, b.String(), args...); err != nil {
					logging.L.Err(err).Msg("inputs batch insert failed")
					return err
				}
			}
			start = end
		}
	}

	// Commit
	if _, err = db.ExecContext(ctx, "COMMIT"); err != nil {
		logging.L.Err(err).Hex("blockhash", utils.ReverseBytes(blockHash)).Int64("height", height).
			Msg("failed to commit db transaction")
		return err
	}
	logging.L.Debug().Hex("blockhash", utils.ReverseBytes(blockHash)).Int64("height", height).
		Msg("successful db commit")
	return nil
}
