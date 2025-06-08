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
			tweak, err := ComputeTweakForTx(txs[i])
			if err != nil {
				return nil, err
			}
			tweaks = append(tweaks, *tweak)
		}
	}

	return tweaks, nil
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
