package indexer

import (
	"context"
	"encoding/hex"

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

func ComputeTweaksForBlock(
	ctx context.Context,
	block BlockData,
) ([]types.Tweak, error) {
	var tweaks []types.Tweak
	txs := block.GetTransactions()

	for i := range txs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !TxHasTaprootOutputs(txs[i]) {
			continue
		}
		tweak, err := ComputeTweakForTx(txs[i])
		if err != nil {
			return nil, err
		}
		tweak.BlockHash = hex.EncodeToString(block.GetBlockHashSlice())
		tweak.BlockHeight = block.GetBlockHeight()
		// logging.L.Debug().Any("tweak", tweak).Msg("Tweak")
		tweaks = append(tweaks, *tweak)
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

	// for _, tweak := range tweaksForBlock {
	// 	fmt.Printf("%s %x\n", tweak.Txid, tweak.TweakData)
	// }

	logging.L.Debug().Msg("Tweaks computed...")
	tweakIndex := BuildPerBlockTweakIndex(blockHash, blockHeight, tweaksForBlock)
	err = dblevel.InsertTweakIndex(tweakIndex)
	if err != nil {
		logging.L.Err(err).Msg("error inserting tweak index")
		return err
	}

	return nil
}

func BuildPerBlockTweakIndex(
	blockHash [32]byte,
	blockHeight uint32,
	tweaks []types.Tweak,
) *types.TweakIndex {
	blockHashString := hex.EncodeToString(blockHash[:])
	tweakData := make([][33]byte, len(tweaks))

	for i := 0; i < len(tweaks); i++ {
		tweakData[i] = tweaks[i].TweakData
	}
	return &types.TweakIndex{
		BlockHash:   blockHashString,
		BlockHeight: blockHeight,
		Data:        tweakData,
	}
}
