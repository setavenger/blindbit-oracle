package indexer

import (
	"context"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

	// Note: forceRebuildStaticIndexesDuringSync removed - static indexes should only be rebuilt with explicit user intention
}

func NewBuilder(ctx context.Context, db database.DB) *Builder {
	logging.L.Info().Msg("Creating indexer builder")

	return &Builder{
		ctx: ctx,
		newBlockChan: make(
			chan *Block,
			config.MaxParallelRequests*20,
		),
		writerChan: make(
			chan *database.DBBlock,
			config.MaxParallelTweakComputations*20,
		),
		store: db,
	}
}

func (b *Builder) ContinuousSync(ctx context.Context) error {
	logging.L.Info().Msg("running continuous sync")
	tickerBlockCheck := time.Tick(3 * time.Second)
	tickerInfo := time.Tick(60 * time.Second)

	if b.newBlockChan == nil {
		b.newBlockChan = make(chan *Block, config.MaxParallelRequests*20)
	}
	if b.writerChan == nil {
		b.writerChan = make(chan *database.DBBlock, config.MaxParallelTweakComputations*20)
	}

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
			logging.L.Info().
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

			// todo: change to single block pull
			// we also check previous blockhash basically going backwards and overwriting if exists
			if uint32(chainInfo.Blocks) > syncTip {
				// +1 because we already processed the tip
				err = b.SingleBlockPullAndHandle(ctx, uint32(chainInfo.Blocks))
				if err != nil {
					logging.L.Err(err).Msg("failed syncing blocks")
					return err
				}

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
	err = b.SyncBlocks(ctx, int64(syncTip)+1, chainInfo.Blocks)
	if err != nil {
		logging.L.Err(err).Msg("failed syncing blocks")
		return err
	}

	return nil
}

func (b *Builder) SyncBlocks(
	ctx context.Context,
	startHeight, endHeight int64,
) error {
	errChan := make(chan error)
	pullSemaphore := make(chan struct{}, config.MaxParallelRequests)
	doneChan := make(chan struct{})
	b.newBlockChan = make(chan *Block, config.MaxParallelRequests*20)
	b.writerChan = make(chan *database.DBBlock, config.MaxParallelTweakComputations*20)

	go func() {
		var wg sync.WaitGroup
		for i := startHeight; i <= endHeight; i++ {
			select {
			case pullSemaphore <- struct{}{}:
				wg.Add(1)
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			logging.L.Trace().Int64("height", i).Msgf("pulling block %d from blockchain", i)
			go func(height int64) {
				defer func() { <-pullSemaphore }() // Release semaphore

				pullStartTime := time.Now()
				err := b.pullBlockToChan(height)
				if err != nil {
					logging.L.Err(err).Int64("height", height).Msg("failed pulling block")
					errChan <- err
					return
				}

				pullTime := time.Since(pullStartTime)
				logging.L.Trace().
					Int64("height", height).
					Dur("pull_time", pullTime).
					Msgf("pulled block %d from blockchain", height)

				wg.Done()
			}(i)
		}
		wg.Wait()
		close(b.newBlockChan)
	}()

	var handleWG sync.WaitGroup
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		handleWG.Add(1)
		workerID := i
		go func() {
			defer handleWG.Done()
			for {
				select {
				case <-ctx.Done():
					logging.L.Trace().Int("worker_id", workerID).Msg("Handler goroutine received context cancellation")
					return
				case block, ok := <-b.newBlockChan:
					if !ok {
						return
					}

					handleStartTime := time.Now()
					logging.L.Trace().
						Int("worker_id", workerID).
						Str("blockhash", block.Hash.String()).
						Int64("height", block.Height).
						Msg("Worker handling block")

					err := b.handleBlock(ctx, block)
					if err != nil {
						logging.L.Err(err).
							Int("worker_id", workerID).
							Str("blockhash", block.Hash.String()).
							Int64("height", block.Height).
							Msg("failed handling block")
						errChan <- err
					}

					handleTime := time.Since(handleStartTime)
					logging.L.Trace().
						Int("worker_id", workerID).
						Str("blockhash", block.Hash.String()).
						Int64("height", block.Height).
						Dur("handle_time", handleTime).
						Msg("Worker completed handling block")
				}
			}
		}()
	}

	// Close writerChan after all handlers finish sending.
	go func() {
		handleWG.Wait()
		close(b.writerChan)
	}()

	go func() {
		tickerReports := time.Tick(500 * time.Millisecond)
		blockHeightTicker := time.Tick(10 * time.Second)

		defer close(doneChan) // signal completion when we exit this goroutine

		for {
			select {
			case <-ctx.Done():
				logging.L.Trace().Msg("Writer goroutine received context cancellation")
				return
			case dbBlock, ok := <-b.writerChan:
				if !ok {
					// channel drained and closed: all done
					return
				}

				err := b.store.ApplyBlock(dbBlock)
				if err != nil {
					logging.L.Err(err).
						Str("blockhash", dbBlock.Hash.String()).
						Uint32("height", dbBlock.Height).
						Msg("failed storing block")
					errChan <- err
					return
				}

				// Build static indexes for this block after storing it
				// Static indexes should always be built for new blocks with integrity checks
				// logging.L.Debug().
				// 	Uint32("height", dbBlock.Height).
				// 	Msg("building static indexes for new block")
				// err = b.store.BuildStaticIndexesForBlock(dbBlock.Hash[:])
				// if err != nil {
				// 	logging.L.Warn().Err(err).
				// 		Str("blockhash", dbBlock.Hash.String()).
				// 		Uint32("height", dbBlock.Height).
				// 		Msg("failed to build static indexes for block")
				// 	// Don't return error here - static indexes are not critical for sync to continue
				// 	// The block data is already stored, so we can continue
				// }

				// Build accelerator indexes for this block after static indexes
				// logging.L.Debug().
				// 	Uint32("height", dbBlock.Height).
				// 	Msg("building accelerator indexes for new block")
				// err = b.store.BuildAcceleratorIndexesForBlock(dbBlock.Hash[:]) // does this actually still build anything necessary? the normal builder should do this already. The is_on_best_chain check is also always going to fail if placed here - apply block is async and blockhash will not be in best chain yet
				// if err != nil {
				// 	logging.L.Err(err).
				// 		Str("blockhash", dbBlock.Hash.String()).
				// 		Uint32("height", dbBlock.Height).
				// 		Msg("failed to build accelerator indexes for block")
				// 	errChan <- err
				// 	return
				// }

				// Simple completion log
				logging.L.Info().
					Uint32("height", dbBlock.Height).
					Msg("block processing completed")

			case <-blockHeightTicker:
				_, syncTip, err := b.store.GetChainTip()
				if err != nil {
					logging.L.Err(err).Msg("failed pulling indexing chain tip")
					errChan <- err
					return
				}

				// Get current blockchain tip for comparison
				chainInfo, err := GetChainInfo()
				if err != nil {
					logging.L.Warn().Err(err).Msg("failed to get chain info for progress report")
					chainInfo = &ChainInfo{Blocks: int64(syncTip)} // fallback
				}

				// Enhanced progress reporting
				logging.L.Info().
					Uint32("indexed_height", syncTip).
					Int64("blockchain_tip", chainInfo.Blocks).
					Int64("blocks_behind", chainInfo.Blocks-int64(syncTip)).
					Int("backlog_chan_pull", len(b.newBlockChan)).
					Int("backlog_chan_db_writer", len(b.writerChan)).
					Int("batch_length", b.store.BatchSize()).
					Msgf("Indexer status: indexed height %d, blockchain tip %d (behind by %d blocks)", syncTip, chainInfo.Blocks, chainInfo.Blocks-int64(syncTip))

				// Additional context about background processing
				if chainInfo.Blocks > int64(syncTip) {
					logging.L.Info().
						Int64("blocks_behind", chainInfo.Blocks-int64(syncTip)).
						Int("blocks_in_pull_queue", len(b.newBlockChan)).
						Int("blocks_in_write_queue", len(b.writerChan)).
						Msg("Background catchup in progress - blocks are being processed below chain tip")
				} else {
					logging.L.Info().Msg("Indexer is up to date with blockchain tip")
				}
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
		case <-doneChan:
			// should probably flush as a defer
			return b.store.FlushBatch(true)
		}
	}
}

func (b *Builder) pullBlockToChan(height int64) error {
	block, err := b.pullBlock(height)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull blockhash")
		return err
	}
	b.newBlockChan <- block
	return err
}

func (b *Builder) pullBlock(height int64) (*Block, error) {
	blockhash, err := getBlockHashByHeight(height)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull blockhash")
		return nil, err
	}
	logging.L.Trace().
		Int64("height", height).
		Str("blockhash", blockhash.String()).
		Msg("pulling block")
	block, err := PullBlockData(blockhash)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull block")
		return nil, err
	}
	block.Height = height
	logging.L.Trace().
		Str("blockhash", blockhash.String()).
		Int64("height", height).
		Msgf("pulled block %d", height)
	return block, err
}

// pullBlockByBlockHash pulls a Block based on the blockhash
// Deprecated: misses height on block
func (b *Builder) pullBlockByBlockHash(blockhash *chainhash.Hash) (*Block, error) {
	panic("not implemented, still needs height")
	// block, err := PullBlockData(blockhash)
	// if err != nil {
	// 	logging.L.Err(err).Str("blockhash", blockhash.String()).Msg("failed to pull block")
	// 	return nil, err
	// }
	// block.Height = height
	// logging.L.Trace().
	// 	Str("blockhash", blockhash.String()).
	// 	Int64("height", height).
	// 	Msgf("pulled block %d", height)
	// return block, err
}

// handleBlock makes all computations for a block, stores it in the database, and computes static indexes
// and sends a DBBlock into the builders writerChan
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

	// Send the block to the writer channel instead of applying directly
	// This ensures proper chain tip updates and batch processing
	select {
	case b.writerChan <- dbBlock:
		logging.L.Trace().
			Str("blockhash", block.Hash.String()).
			Int64("height", block.Height).
			Msg("sent block to writer channel")
	case <-ctx.Done():
		return ctx.Err()
	}

	// Note: Static index computation is now handled by the writer goroutine
	// after the block is properly stored and chain tip is updated
	return nil
}

func handleTx(tx *Transaction) *database.Tx {
	var dbOuts []*database.Output

	// we only want outputs where we know they can be Silent Payments.
	// NO tweak not silent payment
	for i := range tx.outs {
		v := tx.outs[i]
		if bip352.IsP2TR(v.PkScript) {
			dbOuts = append(dbOuts, &database.Output{
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
				Pubkey:    v.prevOut.PkScript[2:],
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
