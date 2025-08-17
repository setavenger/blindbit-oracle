package indexer

import (
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
)

func PullBlockData(blockHash chainhash.Hash) (*Block, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var spentTxOuts [][]*wire.TxOut
	var block *btcutil.Block

	errChan := make(chan error)

	blockHashStr := blockHash.String()
	go func() {
		var err error
		spentTxOuts, err = getSpentUtxos(blockHashStr)
		if err != nil {
			logging.L.Err(err).Msg("failed to pull ")
			errChan <- err
			return
		}
	}()

	go func() {
		var err error
		block, err = getBlockByHash(blockHashStr)
		if err != nil {
			logging.L.Err(err).Msg("failed to pull ")
			errChan <- err
			return
		}
	}()

	wg.Wait()

	select {
	case err := <-errChan:
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
		Hash: b.Hash(),
		txs:  make([]*Transaction, len(spentTxOuts)),
	}

	for i := range len(spentTxOuts) {
		inCount := len(spentTxOuts[i])
		block.txs[i].ins = make([]*Vin, inCount)
		blockTx := b.Transactions()[i]
		for j := range inCount {
			vin := Vin{
				txIn:    blockTx.MsgTx().TxIn[j],
				prevOut: spentTxOuts[i][j],
			}
			block.txs[i].ins[j] = &vin
		}
		block.txs[i].outs = blockTx.MsgTx().TxOut
	}

	return &block, nil
}
