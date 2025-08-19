package indexer

import (
	"context"
	"database/sql"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
	"github.com/setavenger/go-bip352"
)

type Builder struct {
	// blocks are pulled and pushed through the channel to workers to process the blocks
	newBlockChan chan *Block

	// writerChan for single threaded inserts
	writerChan chan *database.DBBlock

	// db connection used for the entire builder in all go routines
	db *sql.DB

	store database.DB
}

const pullSmeaphoreCount = 50
const computeSmeaphoreCount = 20

func NewBuilder(db *sql.DB) *Builder {
	return &Builder{
		newBlockChan: make(chan *Block, pullSmeaphoreCount*100),
		writerChan:   make(chan *database.DBBlock, 250),
		db:           db,
	}
}

func NewBuilderPebble(db *pebble.DB) *Builder {
	return &Builder{
		newBlockChan: make(chan *Block),
		writerChan:   make(chan *database.DBBlock),
		db:           &sql.DB{},
		store: &dbpebble.Store{
			DB: db,
		},
	}
}

func (b *Builder) SyncBlocks(
	ctx context.Context,
	startHeight, endHeight int64,
) error {
	errChan := make(chan error)
	pullSemaphore := make(chan struct{}, pullSmeaphoreCount)
	go func() {
		for i := startHeight; i <= endHeight; i++ {
			select {
			case <-ctx.Done():
				// check here
				errChan <- ctx.Err()
				return
			default:
				logging.L.Info().Msgf("pulling block %d", i)
				func() {
					pullSemaphore <- struct{}{}
					defer func() { <-pullSemaphore }() // Release semaphore
					err := b.pullBlock(i)
					if err != nil {
						errChan <- err
						return
					}
				}()
			}
		}
	}()

	for i := 0; i < computeSmeaphoreCount; i++ {
		go func() {
			for {
				select {
				case block := <-b.newBlockChan:
					err := b.handleBlock(ctx, block)
					if err != nil {
						logging.L.Err(err).
							Str("blockhash", block.Hash.String()).
							Int64("height", block.Height).
							Msg("failed handling block")
						errChan <- err
					}
				}
			}
		}()
	}

	go func() {
		tickerReports := time.Tick(100 * time.Millisecond)

		for {
			select {
			case dbBlock := <-b.writerChan:
				err := b.store.ApplyBlock(dbBlock)
				if err != nil {
					logging.L.Err(err).
						Str("blockhash", dbBlock.Hash.String()).
						Uint32("height", dbBlock.Height).
						Msg("failed storing block")
					errChan <- err
					return
				}

				// if counter > blockBatch {
				// 	err = batcher.Commit()
				// 	if err != nil {
				// 		errChan <- err
				// 		return
				// 	}
				// 	// reset everything for next batch
				// 	counter = 0
				// 	batcher, err = database.BeginBatch(ctx, b.db)
				// 	if err != nil {
				// 		errChan <- err
				// 		return
				// 	}
				// }
				// if err := batcher.InsertBlock(ctx, dbBlock.hash[:], dbBlock.height, dbBlock.txs); err != nil {
				// 	_ = batcher.Rollback()
				// 	logging.L.Err(err).
				// 		Str("blockhash", dbBlock.hash.String()).
				// 		Int64("height", dbBlock.height).
				// 		Msg("failed writing batch")
				// 	errChan <- err
				// 	return
				// }
				//
				// err := database.InsertBlock(ctx, b.db, dbBlock.hash[:], dbBlock.height, dbBlock.txs)
				// if err != nil {
				// 	logging.L.Err(err).
				// 		Str("blockhash", dbBlock.hash.String()).
				// 		Int64("height", dbBlock.height).
				// 		Msg("failed handling block")
				// 	errChan <- err
				// 	return
				// }

			case <-tickerReports:
				logging.L.Warn().
					Int("backlog_chan_pull", len(b.newBlockChan)).
					Int("backlog_chan_db_writer", len(b.writerChan)).
					Msg("new_tick_report")
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// check here as well
			return ctx.Err()
		case err := <-errChan:
			logging.L.Err(err).Msg("there was an error pulling blocks")
			return err
		}
	}
}

func (b *Builder) pullBlock(height int64) error {
	blockhash, err := getBlockHashByHeight(height)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pul blockhash")
		return err
	}
	logging.L.Trace().Str("blockhash", blockhash.String()).Msg("pulling blockhash")
	block, err := PullBlockData(blockhash)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull block")
		return err
	}
	logging.L.Trace().Str("blockhash", blockhash.String()).Msgf("block pulled")
	block.Height = height
	b.newBlockChan <- block
	logging.L.Info().Int64("height", height).Msgf("pulled block %d", height)
	return err
}

// handleBlock makes all computations for a block and sends a DBBlock into the builders writerChan
func (b *Builder) handleBlock(ctx context.Context, block *Block) error {
	logging.L.Debug().
		Str("blockhash", block.Hash.String()).
		Int64("height", block.Height).
		Msg("handling block")
	dbTxs := make([]*database.Tx, len(block.txs))
	for i := range block.txs {
		tx := block.txs[i]
		dbTxs[i] = handleTx(tx)
	}
	logging.L.Debug().
		Str("blockhash", block.Hash.String()).
		Int64("height", block.Height).
		Msg("computation done")
	b.writerChan <- &database.DBBlock{
		Height: uint32(block.Height),
		Hash:   block.Hash,
		Txs:    dbTxs,
	}
	return nil
}

func handleTx(tx *Transaction) *database.Tx {
	tweak, err := ComputeTweakPerTx(tx)
	if err != nil {
		logging.L.Warn().Err(err).
			Str("txid", tx.txid.String()).
			Msg("failed to compute tweak")
		tweak = nil
	}

	// if "82252d8fa50f1e4812dc382a300876bd4b7abb494a0573c750a02c11d50e9a49" == tx.txid.String() {
	// 	logging.L.Warn().Hex("tweak", tweak[:]).Msg("")
	// }

	// get valid inputs

	var dbIns []*database.In
	for i := range tx.ins {
		v := tx.ins[i]
		if bip352.IsP2TR(v.prevOut.PkScript) {
			dbIns = append(dbIns, &database.In{
				SpendTxid: tx.txid[:],
				Idx:       uint32(i), // todo: this should be right
				PrevTxid:  v.txIn.PreviousOutPoint.Hash[:],
				PrevVout:  v.txIn.PreviousOutPoint.Index,
			})
		}
	}

	var dbOuts []*database.Out

	// we only want outputs where we know they can be Silent Payments.
	// NO tweak not silent payment
	if tweak != nil {
		for i := range tx.outs {
			v := tx.outs[i]
			if bip352.IsP2TR(v.PkScript) {
				dbOuts = append(dbOuts, &database.Out{
					Txid:   tx.txid[:],
					Vout:   uint32(i),
					Amount: uint64(v.Value),
					Pubkey: v.PkScript[2:],
				})
			}
		}
	}

	dbTx := database.Tx{
		Txid:  tx.txid[:],
		Tweak: tweak,
		Outs:  dbOuts,
		Ins:   dbIns,
	}

	// if "82252d8fa50f1e4812dc382a300876bd4b7abb494a0573c750a02c11d50e9a49" == tx.txid.String() {
	// 	logging.L.Warn().Any("db_tx", dbTx).Hex("tweak", tweak[:]).Msg("")
	// }

	return &dbTx
}
