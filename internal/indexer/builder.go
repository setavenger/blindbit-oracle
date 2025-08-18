package indexer

import (
	"context"
	"database/sql"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/go-bip352"
)

type Builder struct {
	// blocks are pulled and pushed through the channel to workers to process the blocks
	newBlockChan chan *Block

	// db connection used for the entire builder in all go routines
	db *sql.DB
}

const pullSmeaphoreCount = 20
const computeSmeaphoreCount = 20

func NewBuilder(db *sql.DB) *Builder {
	return &Builder{
		newBlockChan: make(chan *Block, pullSmeaphoreCount*5),
		db:           db,
	}
}

func (b *Builder) SyncBlocks(
	ctx context.Context, startHeight, endHeight int64,
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
	return database.InsertBlock(ctx, b.db, block.Hash[:], block.Height, dbTxs)
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
					Amount: v.Value,
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
