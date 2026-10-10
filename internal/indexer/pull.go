package indexer

import (
	"context"
	"errors"
	"fmt"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"sync"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
)

// PullBlockData fetches a block and its spent outputs concurrently.
//
// Each fetch records its own error and the results are only read after both
// goroutines have finished. A previous version sent errors on an unbuffered
// channel that was only read after wg.Wait(); any single failed request
// therefore blocked its goroutine before wg.Done() and deadlocked the indexer
// permanently.
func PullBlockData(ctx context.Context, blockHash *chainhash.Hash) (*Block, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var spentTxOuts [][]*wire.TxOut
	var block *btcutil.Block
	var spentErr, blockErr error

	blockHashStr := blockHash.String()
	go func() {
		defer wg.Done()
		spentTxOuts, spentErr = getSpentUtxos(ctx, blockHashStr)
		if spentErr != nil {
			logging.L.Err(spentErr).Str("blockhash", blockHashStr).Msg("failed to pull spentutxos")
		}
	}()

	go func() {
		defer wg.Done()
		block, blockErr = getBlockByHash(ctx, blockHashStr)
		if blockErr != nil {
			logging.L.Err(blockErr).Str("blockhash", blockHashStr).Msg("failed to pull block data")
		}
	}()

	wg.Wait()

	if err := errors.Join(spentErr, blockErr); err != nil {
		return nil, err
	}

	return mergeBlockAndSpentTxOuts(block, spentTxOuts)
}

func mergeBlockAndSpentTxOuts(b *btcutil.Block, spentTxOuts [][]*wire.TxOut) (*Block, error) {
	// sense check: assert length is the same
	if len(b.Transactions()) != len(spentTxOuts) {
		return nil, fmt.Errorf("unequal length: %d != %d", len(b.Transactions()), len(spentTxOuts))
	}

	block := Block{
		Hash:          b.Hash(),
		PrevBlockHash: &b.MsgBlock().Header.PrevBlock,
		txs:           make([]*Transaction, len(spentTxOuts)),
	}

	for i := range block.txs {
		v := b.Transactions()[i]
		block.txs[i] = &Transaction{
			txid: v.Hash(),
			outs: v.MsgTx().TxOut,
		}
	}

	for i := range len(spentTxOuts) {
		inCount := len(spentTxOuts[i])

		orgBlockTx := b.Transactions()[i]
		block.txs[i] = &Transaction{
			txid: orgBlockTx.Hash(),
			outs: orgBlockTx.MsgTx().TxOut,
		}
		block.txs[i].ins = make([]*Vin, inCount)

		for j := range inCount {
			vin := Vin{
				txIn:    orgBlockTx.MsgTx().TxIn[j],
				prevOut: spentTxOuts[i][j],
			}
			block.txs[i].ins[j] = &vin
		}
	}

	return &block, nil
}

// SingleBlockPullAndHandle pulls a block and processes it.
// If the previous blockhash is not in the db it will recursively pull the previous block.
// Intended to handle reorgs not full chain syncs.
func (b *Builder) SingleBlockPullAndHandle(
	ctx context.Context, height uint32,
) error {
	block, err := b.pullBlock(ctx, int64(height))
	if err != nil {
		return err
	}

	err = b.handleBlock(ctx, block)
	if err != nil {
		return err
	}

	blockhashInDB, err := b.store.BlockhashInDB(block.PrevBlockHash[:])
	if err != nil {
		logging.L.Err(err).
			Str("prev_blockhash", block.Hash.String()).
			Msg("error hen trying to look up previous block hash")
		return err
	}
	// Stop at genesis and at the operator's configured start height. `height` is
	// unsigned: without the height > 0 guard, height-1 at genesis wraps to
	// 4294967295 and the node answers 400 for that blockhash, which rest.go
	// treats as fatal. On a fresh database the previous hash is never in the DB,
	// so this recursion walks the whole chain down and would always underflow.
	if !blockhashInDB && height > 0 && height > config.SyncStartHeight {
		// do previous block as well
		err = b.SingleBlockPullAndHandle(ctx, height-1)
		if err != nil {
			logging.L.Err(err).
				Str("blockhash", block.Hash.String()).
				Str("blockhash_prev", block.PrevBlockHash.String()).
				Msg("failed ot pull previous block")
			return err
		}
	}

	return nil
}
