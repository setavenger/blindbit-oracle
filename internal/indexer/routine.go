package indexer

import (
	"context"
	"sync"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

// func IndexBlock(
// 	ctx context.Context,
// 	blockHash [32]byte,
// 	blockHeight uint32,
// 	txs []Transaction,
// ) error {
// }

var numTweakWorkers = 12 // number of concurrent workers for tweak computation

func ComputeTweaksForBlock(
	ctx context.Context,
	block BlockData,
) ([]types.Tweak, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	txs := block.GetTransactions()

	// Create channels for results and semaphore
	type result struct {
		tweak *types.Tweak
		err   error
	}
	resultChan := make(chan result, len(txs))
	semaphore := make(chan struct{}, numTweakWorkers)
	var wg sync.WaitGroup

	// Process transactions in parallel
	for i := range txs {
		// we only compute tweaks for transactions with taproot outputs
		if !TxHasTaprootOutputs(txs[i]) {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore
		go func(tx Transaction) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			tweak, err := ComputeTweakForTx(tx)
			if err != nil {
				resultChan <- result{err: err}
				return
			}

			tweak.BlockHash = block.GetBlockHash()
			tweak.BlockHeight = block.GetBlockHeight()
			resultChan <- result{tweak: tweak}
		}(txs[i])
	}

	// Start a goroutine to close the result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var tweaks []types.Tweak
	for result := range resultChan {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if result.err != nil {
				return nil, result.err
			}
			if result.tweak != nil {
				tweaks = append(tweaks, *result.tweak)
			}
		}
	}

	return tweaks, nil
}

func TxHasTaprootOutputs(tx Transaction) bool {
	txOuts := tx.GetTxOuts()
	for _, txOut := range txOuts {
		pkScript := txOut.GetPkScript()
		// pkScripts for taproot outputs are exactly 34 bytes
		if len(pkScript) != 34 {
			continue
		}
		if IsP2TR(pkScript) {
			return true
		}
	}
	return false
}

func HandleBlock(
	ctx context.Context,
	block BlockData,
) error {
	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	logging.L.Debug().Msg("Computing tweaks...")
	blockHash := block.GetBlockHash()
	blockHeight := block.GetBlockHeight()

	tweaksForBlock, err := ComputeTweaksForBlock(ctx, block)
	if err != nil {
		logging.L.Err(err).Msg("error computing tweaks")
		return err
	}

	tweakIndex := BuildPerBlockTweakIndex(blockHash, blockHeight, tweaksForBlock)
	err = dblevel.InsertTweakIndex(tweakIndex)
	if err != nil {
		logging.L.Err(err).Msg("error inserting tweak index")
		return err
	}

	tweakIndexDust := BuildPerBlockTweakIndexDust(blockHash, blockHeight, tweaksForBlock)

	err = dblevel.InsertTweakIndexDust(tweakIndexDust)
	if err != nil {
		logging.L.Err(err).Msg("error inserting tweak index dust")
		return err
	}

	return nil
}

func BuildPerBlockTweakIndex(
	blockHash [32]byte,
	blockHeight uint32,
	tweaks []types.Tweak,
) *types.TweakIndex {
	tweakData := make([][33]byte, len(tweaks))

	for i := 0; i < len(tweaks); i++ {
		tweakData[i] = tweaks[i].TweakData
	}

	return &types.TweakIndex{
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
		Data:        tweakData,
	}
}

func BuildPerBlockTweakIndexDust(
	blockHash [32]byte,
	blockHeight uint32,
	tweaks []types.Tweak,
) *types.TweakIndexDust {
	tweakData := make([]types.TweakDusted, len(tweaks))

	for i := range tweaks {
		tweakData[i] = types.TweakData{
			Data:  tweaks[i].TweakData,
			Value: tweaks[i].HighestValue,
		}
	}

	return &types.TweakIndexDust{
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
		Data:        tweakData,
	}
}
