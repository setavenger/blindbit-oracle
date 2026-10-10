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

// progressReportInterval is how often SyncBlocks logs its progress, which
// includes asking the node for its chain state. A variable so tests can
// shorten it.
var progressReportInterval = 10 * time.Second

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

func (b *Builder) ContinuousSync(parent context.Context) error {
	logging.L.Info().Msg("running continuous sync")

	// The writer cancels ctx when it fails, which unblocks a handleBlock that is
	// waiting to hand it a block. Without this the main loop could block on
	// writerChan while the writer blocks reporting its error: a deadlock.
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	tickerBlockCheck := time.Tick(3 * time.Second)
	tickerInfo := time.Tick(60 * time.Second)

	// recreate the channels, as they might have been closed by previous SyncBlocks
	b.newBlockChan = make(chan *Block, config.MaxParallelRequests*20)
	b.writerChan = make(chan *database.DBBlock, config.MaxParallelTweakComputations*20)

	// Start writer goroutine to process blocks from writerChan.
	// errChan is buffered: the writer sends at most one error and then exits,
	// so the send never blocks even if the main loop has already returned.
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})
	// writerErr prefers the writer's own error over the cancellation it caused.
	writerErr := func(fallback error) error {
		select {
		case err := <-errChan:
			return err
		default:
			return fallback
		}
	}

	go func() {
		defer close(doneChan)
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
					cancel()
					return
				}

				// need to flush as batch sizes won't get big here
				// only reorgs but even those are small usually
				err = b.store.FlushBatch(true)
				if err != nil {
					logging.L.Err(err).Msg("failed flushing batch")
					errChan <- err
					cancel()
					return
				}

				logging.L.Info().
					Uint32("height", dbBlock.Height).
					Msg("block processing completed")
			}
		}
	}()

	// On return, stop the writer and wait for it so it never outlives the call.
	defer func() {
		cancel()
		<-doneChan
	}()

	for {
		select {
		case <-ctx.Done():
			return writerErr(ctx.Err())
		case err := <-errChan:
			logging.L.Err(err).Msg("there was an error processing blocks")
			return err
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

			chainInfo, err := GetChainInfo(ctx)
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
					err = writerErr(err)
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

	chainInfo, err := GetChainInfo(ctx)
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

// SyncBlocks indexes the heights in [startHeight, endHeight].
//
// Three stages are connected by channels: pullers fetch blocks from the node
// (at most MaxParallelRequests at a time), workers compute the tweaks and a
// single writer stores the results. The first error from any stage cancels the
// others and is returned. Errors are recorded without blocking, so several
// stages may fail at the same time, and every channel send also selects on the
// context, so a stage can never wait forever on a peer that has stopped.
//
// SyncBlocks only returns after every goroutine it started has exited: none of
// them outlives the call or touches the builder's channels afterwards.
func (b *Builder) SyncBlocks(
	parent context.Context,
	startHeight, endHeight int64,
) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var (
		errOnce  sync.Once
		firstErr error
	)
	// fail records the first error and stops every stage. It never blocks.
	// cancel() is only called here, after the error has been recorded, so an
	// error that merely results from this cancellation can never be recorded
	// first; context.Canceled wins only when the caller cancelled.
	fail := func(err error) {
		errOnce.Do(func() { firstErr = err })
		cancel()
	}

	b.newBlockChan = make(chan *Block, config.MaxParallelRequests*20)
	b.writerChan = make(chan *database.DBBlock, config.MaxParallelTweakComputations*20)
	newBlockChan, writerChan := b.newBlockChan, b.writerChan

	var stages sync.WaitGroup

	// Pullers. newBlockChan is closed once every started pull has returned.
	stages.Add(1)
	go func() {
		defer stages.Done()

		var pullers sync.WaitGroup
		defer func() {
			pullers.Wait()
			close(newBlockChan)
		}()

		pullSemaphore := make(chan struct{}, config.MaxParallelRequests)
		for i := startHeight; i <= endHeight; i++ {
			select {
			case pullSemaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}

			pullers.Add(1)
			logging.L.Trace().Int64("height", i).Msgf("pulling block %d from blockchain", i)
			go func(height int64) {
				defer pullers.Done()
				defer func() { <-pullSemaphore }() // Release semaphore

				pullStartTime := time.Now()
				err := b.pullBlockToChan(ctx, height)
				if err != nil {
					if ctx.Err() == nil {
						logging.L.Err(err).Int64("height", height).Msg("failed pulling block")
					}
					fail(err)
					return
				}

				pullTime := time.Since(pullStartTime)
				logging.L.Trace().
					Int64("height", height).
					Dur("pull_time", pullTime).
					Msgf("pulled block %d from blockchain", height)
			}(i)
		}
	}()

	// Workers. writerChan is closed once every worker has returned.
	var workers sync.WaitGroup
	for i := 0; i < config.MaxParallelTweakComputations; i++ {
		workers.Add(1)
		workerID := i
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					logging.L.Trace().Int("worker_id", workerID).Msg("Handler goroutine received context cancellation")
					return
				case block, ok := <-newBlockChan:
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
						if ctx.Err() == nil {
							logging.L.Err(err).
								Int("worker_id", workerID).
								Str("blockhash", block.Hash.String()).
								Int64("height", block.Height).
								Msg("failed handling block")
						}
						fail(err)
						return
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

	stages.Add(1)
	go func() {
		defer stages.Done()
		workers.Wait()
		close(writerChan)
	}()

	// Writer. Single goroutine so that blocks are applied one at a time.
	stages.Add(1)
	go func() {
		defer stages.Done()

		tickerReports := time.Tick(500 * time.Millisecond)
		blockHeightTicker := time.Tick(progressReportInterval)

		for {
			select {
			case <-ctx.Done():
				logging.L.Trace().Msg("Writer goroutine received context cancellation")
				return
			case dbBlock, ok := <-writerChan:
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
					fail(err)
					return
				}

				// Simple completion log
				logging.L.Info().
					Uint32("height", dbBlock.Height).
					Msg("block processing completed")

			case <-blockHeightTicker:
				_, syncTip, err := b.store.GetChainTip()
				if err != nil {
					logging.L.Err(err).Msg("failed pulling indexing chain tip")
					fail(err)
					return
				}

				// Get current blockchain tip for comparison
				chainInfo, err := GetChainInfo(ctx)
				if err != nil {
					logging.L.Warn().Err(err).Msg("failed to get chain info for progress report")
					chainInfo = &ChainInfo{Blocks: int64(syncTip)} // fallback
				}

				// Enhanced progress reporting
				logging.L.Info().
					Uint32("indexed_height", syncTip).
					Int64("blockchain_tip", chainInfo.Blocks).
					Int64("blocks_behind", chainInfo.Blocks-int64(syncTip)).
					Int("backlog_chan_pull", len(newBlockChan)).
					Int("backlog_chan_db_writer", len(writerChan)).
					Int("batch_length", b.store.BatchSize()).
					Msgf("Indexer status: indexed height %d, blockchain tip %d (behind by %d blocks)", syncTip, chainInfo.Blocks, chainInfo.Blocks-int64(syncTip))

				// Additional context about background processing
				if chainInfo.Blocks > int64(syncTip) {
					logging.L.Info().
						Int64("blocks_behind", chainInfo.Blocks-int64(syncTip)).
						Int("blocks_in_pull_queue", len(newBlockChan)).
						Int("blocks_in_write_queue", len(writerChan)).
						Msg("Background catchup in progress - blocks are being processed below chain tip")
				} else {
					logging.L.Info().Msg("Indexer is up to date with blockchain tip")
				}
			case <-tickerReports:
				logging.L.Trace().
					Int("backlog_chan_pull", len(newBlockChan)).
					Int("backlog_chan_db_writer", len(writerChan)).
					Msg("new_tick_report")
			}
		}
	}()

	stages.Wait()

	if firstErr != nil {
		logging.L.Err(firstErr).Msg("there was an error pulling blocks")
		return firstErr
	}
	if err := parent.Err(); err != nil {
		return err
	}
	// should probably flush as a defer
	return b.store.FlushBatch(true)
}

func (b *Builder) pullBlockToChan(ctx context.Context, height int64) error {
	block, err := b.pullBlock(ctx, height)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull blockhash")
		return err
	}
	select {
	case b.newBlockChan <- block:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Builder) pullBlock(ctx context.Context, height int64) (*Block, error) {
	blockhash, err := getBlockHashByHeight(ctx, height)
	if err != nil {
		logging.L.Err(err).Int64("height", height).Msg("failed to pull blockhash")
		return nil, err
	}
	logging.L.Trace().
		Int64("height", height).
		Str("blockhash", blockhash.String()).
		Msg("pulling block")
	block, err := PullBlockData(ctx, blockhash)
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
	// The send must also watch ctx: if the writer has stopped (error or
	// cancellation) nobody drains writerChan and a plain send blocks forever.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.writerChan <- dbBlock:
		logging.L.Trace().
			Str("blockhash", block.Hash.String()).
			Int64("height", block.Height).
			Msg("sent block to writer channel")
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
