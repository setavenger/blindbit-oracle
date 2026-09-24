package indexer

import (
	"bytes"
	"context"
	"fmt"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"sync"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
)

func PullBlockData(blockHash *chainhash.Hash) (*Block, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var spentTxOuts [][]*wire.TxOut
	var block *btcutil.Block

	errChan := make(chan error)

	blockHashStr := blockHash.String()
	go func() {
		defer wg.Done()
		var err error
		spentTxOuts, err = getSpentUtxos(blockHashStr)
		if err != nil {
			logging.L.Err(err).Str("blockhash", blockHashStr).Msg("failed to pull spentutxos")
			errChan <- err
			return
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		block, err = getBlockByHash(blockHashStr)
		if err != nil {
			logging.L.Err(err).Str("blockhash", blockHashStr).Msg("failed to pull block data")
			errChan <- err
			return
		}
	}()

	wg.Wait()

	select {
	case err := <-errChan:
		logging.L.Err(err).Msg("ended with err")
		return nil, err
	default:
		// No errors
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

// SingleBlockPullAndHandle brings the index up to the node's block at height.
//
// It first finds the fork point: the highest height at or below both the
// node's height and the index tip where the index holds the same block as the
// node. Every height above it is then pulled from the node and applied in
// ascending order, one block at a time. A height that already holds a
// different block is replaced by the store, which removes everything the
// displaced block contributed (see dbpebble.Store.ApplyBlock). In the common
// case the index tip matches the node and only the new blocks are pulled.
func (b *Builder) SingleBlockPullAndHandle(
	ctx context.Context, height uint32,
) error {
	fork, prevHash, err := b.findForkPoint(height)
	if err != nil {
		return err
	}
	for h := fork + 1; h <= int64(height); h++ {
		block, err := b.pullBlock(h)
		if err != nil {
			return err
		}
		// The node can switch branches while we pull. A block that does not
		// build on the one applied before it means exactly that: stop here and
		// let the next check find the new fork point.
		if prevHash != nil && !bytes.Equal(block.PrevBlockHash[:], prevHash) {
			logging.L.Warn().
				Int64("height", h).
				Str("blockhash", block.Hash.String()).
				Str("prev_blockhash", block.PrevBlockHash.String()).
				Msg("node changed branch during catch-up; retrying on next check")
			return nil
		}
		if err := b.handleBlock(ctx, block); err != nil {
			return err
		}
		prevHash = block.Hash[:]
	}

	return nil
}

// findForkPoint returns the highest height h <= min(height, index tip), and
// not below the configured start height, at which the index holds the node's
// block, together with that block's hash. If there is none it returns
// SyncStartHeight-1 and a nil hash, so everything from the start height is
// (re)applied.
//
// Heights are compared by hash rather than by looking up the parent hash in
// the blockhash->height index, so an orphaned block that is still indexed
// somewhere can never end the walk-back early.
func (b *Builder) findForkPoint(height uint32) (int64, []byte, error) {
	_, dbTip, err := b.store.GetChainTip()
	if err != nil {
		return 0, nil, err
	}
	top := int64(min(height, dbTip))
	for h := top; h >= int64(config.SyncStartHeight); h-- {
		indexed, err := b.store.GetBlockHashByHeight(uint32(h))
		if err != nil {
			return 0, nil, err
		}
		if indexed == nil {
			continue
		}
		nodeHash, err := getBlockHashByHeight(h)
		if err != nil {
			return 0, nil, err
		}
		if bytes.Equal(indexed, nodeHash[:]) {
			if h < top {
				logging.L.Warn().
					Int64("fork_height", h).
					Int64("indexed_up_to", top).
					Msg("reorg: index diverges from the node above the fork height")
			}
			return h, indexed, nil
		}
	}
	return int64(config.SyncStartHeight) - 1, nil, nil
}
