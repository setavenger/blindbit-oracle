package indexer

import (
	"context"
	"errors"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

// Cleamup iterates from config syncstartheight to tip and
// checks for missing blocks to pull those and patch missing block data
func (b *Builder) DBIntegrityCheck() error {
	_, syncTip, err := b.store.GetChainTip()
	if err != nil {
		return err
	}

	logging.L.Info().
		Uint32("config_sync_start_height", config.SyncStartHeight).
		Uint32("sync_tip", syncTip).
		Msg("syncing heights")

	for height := config.SyncStartHeight; height <= syncTip; height++ {
		// todo: turn into more efficient sync blocks (concurrent)
		// if somehow big gaps become apparent
		blockhash, err := b.store.GetBlockHashByHeight(height)
		if err != nil && !errors.Is(err, pebble.ErrNotFound) {
			return err
		}

		if blockhash != nil {
			// block is already processed
			continue
		}

		logging.L.Info().
			Uint32("height", height).
			Msg("block is missing, trying to process block")
		err = b.singleProcessBlock(b.ctx, height)
		if err != nil {
			logging.L.Err(err).Uint32("height", height).Msg("failed to process block")
			return err
		}
	}

	err = b.store.FlushBatch(true)
	if err != nil {
		logging.L.Err(err).Msg("failed flushing batch")
		return err
	}

	return nil
}

// singleProcessBlock pulls a block and processes it
func (b *Builder) singleProcessBlock(ctx context.Context, height uint32) error {
	wg := sync.WaitGroup{}
	wg.Add(2)
	errChan := make(chan error)
	go func() {
		defer wg.Done()
		err := b.pullBlock(int64(height))
		if err != nil {
			errChan <- err
			return
		}
	}()
	go func() {
		defer wg.Done()
		block := <-b.newBlockChan
		err := b.handleBlock(ctx, block)
		if err != nil {
			errChan <- err
			return
		}
	}()

	wg.Wait()
	select {
	case err := <-errChan:
		return err
	default:
		// No errors
	}

	return nil
}
