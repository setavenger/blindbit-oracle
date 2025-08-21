package indexer

import (
	"context"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/go-bip352"
)

type Builder struct {
	ctx context.Context

	// blocks are pulled and pushed through the channel to workers to process the blocks
	newBlockChan chan *Block

	// writerChan for single threaded inserts
	writerChan chan *database.DBBlock

	// db connection used for the entire builder in all go routines
	store database.DB
}

func NewBuilder(ctx context.Context, db database.DB) *Builder {
	return &Builder{
		ctx:          ctx,
		newBlockChan: make(chan *Block, config.MaxParallelRequests*20),
		writerChan:   make(chan *database.DBBlock, config.MaxParallelTweakComputations*20),
		store:        db,
	}
}

func (b *Builder) ContinuousSync(ctx context.Context) error {
	tickerBlockCheck := time.Tick(3 * time.Second)
	tickerInfo := time.Tick(15 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerInfo:
			blockhash, syncTip, err := b.store.GetChainTip()
			if err != nil {
				logging.L.Err(err).Msg("failed to pull chain tip from db")
				return err
			}
			logging.L.Debug().
				Hex("best_blockhash", utils.ReverseBytesCopy(blockhash)).
				Uint32("height", syncTip).
				Msg("state_update")

		case <-tickerBlockCheck:
			_, syncTip, err := b.store.GetChainTip()
			if err != nil {
				logging.L.Err(err).Msg("failed to pull chain tip from db")
				return err
			}

			chainInfo, err := GetChainInfo()
			if err != nil {
				logging.L.Err(err).Msg("failed to pull chainInfo")
				return err
			}
			// use chainInfo.BestBlockhash to determine a reorg
			if uint32(chainInfo.Blocks) > syncTip {
				b.SyncBlocks(ctx, int64(syncTip), chainInfo.Blocks)
			}
		}
	}
}

func (b *Builder) InitialSyncToTip(
	ctx context.Context,
) error {
	_, syncTip, err := b.store.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("failed to pull chain tip from db")
		return err
	}

	chainInfo, err := GetChainInfo()
	if err != nil {
		logging.L.Err(err).Msg("failed to pull chainInfo")
		return err
	}

	// we either start off where the user said or where we were last
	// otherwise we end up reindexing.
	// todo: Add a check to see whether all blocks have been indexed
	// todo: Inegrity check no gaps in index, re-orgs or whatever
	syncTip = max(syncTip, config.SyncStartHeight)

	logging.L.Info().
		Uint32("syncTip", syncTip).
		Int64("chaintip", chainInfo.Blocks).
		Msg("Starting initial sync")

	// using Blocks which is the actual count of blocks the node has available (assumption)
	return b.SyncBlocks(ctx, int64(syncTip), chainInfo.Blocks)
}

func (b *Builder) SyncBlocks(
	ctx context.Context,
	startHeight, endHeight int64,
) error {
	errChan := make(chan error)
	pullSemaphore := make(chan struct{}, config.MaxParallelRequests)

	go func() {
		for i := startHeight; i <= endHeight; i++ {
			select {
			case pullSemaphore <- struct{}{}:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			logging.L.Trace().Int64("height", i).Msgf("pulling block %d", i)
			go func(height int64) {
				defer func() { <-pullSemaphore }() // Release semaphore
				err := b.pullBlock(height)
				if err != nil {
					errChan <- err
					return
				}
			}(i)
		}
	}()

	for i := 0; i < config.MaxParallelTweakComputations; i++ {
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
		tickerReports := time.Tick(500 * time.Millisecond)
		blockHeightTicker := time.Tick(10 * time.Second)

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

			case <-blockHeightTicker:
				_, syncTip, err := b.store.GetChainTip()
				if err != nil {
					logging.L.Err(err).Msg("failed pulling indexing chain tip")
					errChan <- err
					return
				}

				logging.L.Info().
					Uint32("height", syncTip).
					Msgf("current indexed chain tip %d", syncTip)
				logging.L.Info().
					Int("backlog_chan_pull", len(b.newBlockChan)).
					Int("backlog_chan_db_writer", len(b.writerChan)).
					Msg("new_tick_report")
			case <-tickerReports:
				logging.L.Trace().
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
	logging.L.Trace().
		Int64("height", height).
		Str("blockhash", blockhash.String()).
		Msg("pulling block")
	block, err := PullBlockData(blockhash)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull block")
		return err
	}
	block.Height = height
	b.newBlockChan <- block
	logging.L.Trace().
		Str("blockhash", blockhash.String()).
		Int64("height", height).
		Msgf("pulled block %d", height)
	return err
}

// handleBlock makes all computations for a block and sends a DBBlock into the builders writerChan
func (b *Builder) handleBlock(ctx context.Context, block *Block) error {
	logging.L.Trace().
		Str("blockhash", block.Hash.String()).
		Int64("height", block.Height).
		Msg("handling block")
	dbTxs := make([]*database.Tx, len(block.txs))
	for i := range block.txs {
		tx := block.txs[i]
		// we also insert nils so we can pinpoint the positions later on
		// we skip nils in insert logic
		dbTxs[i] = handleTx(tx)
	}
	logging.L.Trace().
		Str("blockhash", block.Hash.String()).
		Int64("height", block.Height).
		Msg("computation done")

	dbBlock := &database.DBBlock{
		Height: uint32(block.Height),
		Hash:   block.Hash,
		Txs:    dbTxs,
	}

	// b.writerChan <- dbBlock
	return b.store.ApplyBlock(dbBlock)
	// return nil
}

func handleTx(tx *Transaction) *database.Tx {
	var dbOuts []*database.Out

	// we only want outputs where we know they can be Silent Payments.
	// NO tweak not silent payment
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

	var err error
	var tweak *[33]byte
	if len(dbOuts) > 0 {
		tweak, err = ComputeTweakPerTx(tx)
		if err != nil {
			logging.L.Warn().Err(err).
				Str("txid", tx.txid.String()).
				Msg("failed to compute tweak")
			tweak = nil
		}
	}

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

	// if no data of interest is in the tx we skip
	if tweak != nil || len(dbOuts) > 0 || len(dbIns) > 0 {
		dbTx := database.Tx{
			Txid:  tx.txid[:],
			Tweak: tweak,
			Outs:  dbOuts,
			Ins:   dbIns,
		}
		return &dbTx
	}

	return nil
}
