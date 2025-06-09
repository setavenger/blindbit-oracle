package bitcoinkernel

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
	"github.com/setavenger/go-bitcoinkernel/pkg/bitcoinkernel"
)

var (
	// Bitcoin Core Data Directory
	datadir string

	kernelContext *bitcoinkernel.Context
)

func init() {
	ctx, err := bitcoinkernel.NewContext()
	if err != nil {
		logging.L.Err(err).Msg("Failed to create context")
	}
	kernelContext = ctx
}

func SetDatadir(dir string) {
	datadir = dir
}

func SyncToTipFromHeight(
	ctx context.Context,
	startHeight uint32,
	endHeight *uint32,
) error {
	if datadir == "" {
		logging.L.Fatal().Msg("bitcoin core datadir not set")
	}
	logging.L.Info().Msgf("Syncing from block height: %d", startHeight)
	if endHeight != nil {
		logging.L.Info().Msgf("Syncing to block height: %d", *endHeight)
	} else {
		logging.L.Info().Msg("Syncing to tip")
	}

	defer kernelContext.Close()

	logging.L.Info().Msg("Creating chainstate manager")
	chainman, err := bitcoinkernel.NewChainstateManager(kernelContext, datadir)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create chainstate manager")
		return err
	}
	defer chainman.Close()
	logging.L.Info().Msgf("Loaded chainstate manager")

	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	blockIndex, err := chainman.GetBlockIndexFromHeight(int(startHeight))
	if err != nil {
		logging.L.Err(err).Msg("Failed to get block index from height")
		return err
	}

	for blockIndex != nil && (endHeight == nil || blockIndex.GetHeight() < *endHeight) {
		err = SyncBlockIndex(cctx, chainman, blockIndex)
		if err != nil {
			logging.L.Err(err).Msg("Failed to sync block index")
			return err
		}
		blockIndex = chainman.GetNextBlockIndex(blockIndex)
	}

	return nil
}

func SyncBlockIndex(
	ctx context.Context,
	chainman *bitcoinkernel.ChainstateManager,
	blockIndex *bitcoinkernel.BlockIndex,
) error {
	logging.L.Info().Msgf("Syncing block index: %d", blockIndex.GetHeight())

	blockKernel, err := chainman.ReadBlockData(blockIndex)
	if err != nil {
		log.Fatalf("Failed to read block data: %v", err)
	}
	defer blockKernel.Close()
	blockData, err := blockKernel.GetData()
	if err != nil {
		log.Fatalf("Failed to get block data: %v", err)
	}

	// Read block undo data
	blockUndo, err := chainman.ReadUndoData(blockIndex)
	if err != nil {
		logging.L.Err(err).Msg("Failed to read block undo data")
		return err
	}
	defer blockUndo.Close()

	block := NewKernelBlock(blockData, uint32(blockIndex.GetHeight()), blockUndo)
	if block == nil {
		err = errors.New("failed to create kernel block")
		logging.L.Err(err).Msg("Failed to create kernel block")
		return err
	}

	err = indexer.HandleBlock(ctx, block)
	if err != nil {
		logging.L.Err(err).Msg("Failed to compute tweaks")
		return err
	}

	return nil
}

// SyncHeight indexes the block with the given height
func SyncHeight(
	ctx context.Context,
	blockHeight uint32,
) error {
	if datadir == "" {
		logging.L.Fatal().Msg("bitcoin core datadir not set")
	}
	logging.L.Info().Msgf("Syncing block height: %d", blockHeight)

	defer kernelContext.Close()
	chainman, err := bitcoinkernel.NewChainstateManager(kernelContext, datadir)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create chainstate manager")
		return err
	}
	defer chainman.Close()

	blockIndex, err := chainman.GetBlockIndexFromHeight(int(blockHeight))
	if err != nil {
		logging.L.Err(err).Msg("Failed to get block index from height")
		return err
	}

	return SyncBlockIndex(ctx, chainman, blockIndex)
}
