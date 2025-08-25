package database

type Out struct {
	Txid   []byte
	Vout   uint32
	Amount int64
	Pubkey []byte
}

type In struct {
	SpendTxid []byte
	Idx       uint32
	PrevTxid  []byte
	PrevVout  uint32
}

type Tx struct {
	Txid  []byte
	Tweak *[33]byte // nil if not relevant
	Outs  []*Out    // only Taproot outs you index
	Ins   []*In     // inputs for spend-index (spends of your tracked outs)
}

// func InsertBlock(
// 	ctx context.Context, db *sql.DB, blockHash []byte, height int64, txs []*Tx,
// ) error {
// 	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	defer func() {
// 		if err != nil {
// 			_ = tx.Rollback()
// 		}
// 	}()
//
// 	// Ensure header + best-chain mapping
// 	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO headers(block_hash) VALUES (?)`, blockHash); err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
// 	// Upsert best-chain height (you’ll update this when reorgs happen)
// 	if _, err = tx.ExecContext(ctx, `INSERT INTO chain_index(block_height, block_hash) VALUES (?,?)
// 		ON CONFLICT(block_height) DO UPDATE SET block_hash=excluded.block_hash`, height, blockHash); err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
//
// 	insTx, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO transactions(txid, tweak) VALUES(?, ?)`)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
// 	defer insTx.Close()
//
// 	insBT, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO block_txs(block_hash, position, txid) VALUES(?, ?, ?)`)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
// 	defer insBT.Close()
//
// 	insOut, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO outputs(txid, vout, amount) VALUES(?, ?, ?)`)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
// 	defer insOut.Close()
//
// 	insIn, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO inputs(spend_txid, idx, prev_txid, prev_vout) VALUES(?, ?, ?, ?)`)
// 	if err != nil {
// 		logging.L.Err(err).Msg("failed insert")
// 		return err
// 	}
// 	defer insIn.Close()
//
// 	for pos, t := range txs {
// 		// record tx occurrence in block
// 		if _, err = insBT.Exec(blockHash, pos, t.Txid); err != nil {
// 			logging.L.Err(err).Msg("failed insert")
// 			return err
// 		}
//
// 		// if it has a tweak, store it once
// 		if t.Tweak != nil {
// 			if _, err = insTx.Exec(t.Txid, t.Tweak[:]); err != nil {
// 				logging.L.Err(err).Msg("failed insert")
// 				return err
// 			}
// 		}
//
// 		// outputs you care about
// 		for _, o := range t.Outs {
// 			if _, err = insOut.Exec(o.Txid, o.Vout, o.Amount); err != nil {
// 				logging.L.Err(err).Any("output", o).
// 					Hex("txid", o.Txid).
// 					Hex("txid_rev", utils.ReverseBytesCopy(o.Txid)).
// 					Msg("failed insert")
// 				return err
// 			}
// 		}
// 		// spend events (spenders of your tracked outs)
// 		for _, in := range t.Ins {
// 			if _, err = insIn.Exec(in.SpendTxid, in.Idx, in.PrevTxid, in.PrevVout); err != nil {
// 				logging.L.Err(err).Msg("failed insert")
// 				return err
// 			}
// 		}
// 	}
//
// 	err = tx.Commit()
// 	if err != nil {
// 		logging.L.Err(err).
// 			Hex("blockhash", utils.ReverseBytes(blockHash)).
// 			Int64("height", height).
// 			Msg("failed to commit db transaction")
// 		return err
// 	}
//
// 	// todo: align blockhashes such that we have consistency in logging
// 	logging.L.Debug().
// 		Hex("blockhash", utils.ReverseBytes(blockHash)).
// 		Int64("height", height).
// 		Msg("succesfull db commit")
// 	return err
// }
