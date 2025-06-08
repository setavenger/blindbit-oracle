package bitcoinkernel

import (
	"context"
	"log"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
	"github.com/setavenger/go-bitcoinkernel/pkg/bitcoinkernel"
)

var (
	// Bitcoin Core Data Directory
	datadir string
	height  int

	kernelContext *bitcoinkernel.Context
)

func init() {
	ctx, err := bitcoinkernel.NewContext()
	if err != nil {
		logging.L.Err(err).Msg("Failed to create context")
	}
	kernelContext = ctx
}

// SyncHeight indexes the block with the given height
func SyncHeight(
	ctx context.Context,
	blockHeight uint32,
) error {
	defer kernelContext.Close()
	chainman, err := bitcoinkernel.NewChainstateManager(kernelContext, datadir)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create chainstate manager")
		return err
	}
	defer chainman.Close()

	blockIndex, err := chainman.GetBlockIndexFromHeight(int(blockHeight))
	if err != nil {
		logging.L.Err(err).Msg("Failed to get block by height")
		return err
	}
	if err != nil {
		log.Fatalf("Failed to get block index: %v", err)
	}

	defer blockIndex.Close()

	blockKernel, err := chainman.ReadBlockData(blockIndex)
	if err != nil {
		log.Fatalf("Failed to read block data: %v", err)
	}
	defer blockKernel.Close()
	blockData, err := blockKernel.GetData()
	if err != nil {
		log.Fatalf("Failed to get block data: %v", err)
	}
	block := NewKernelBlock(blockData, blockHeight)
	err = indexer.HandleBlock(context.Background(), block)
	if err != nil {
		logging.L.Err(err).Msg("Failed to compute tweaks")
		return err
	}

	return nil
}
