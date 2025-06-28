package bitcoinkernel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
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
	ctx, err := bitcoinkernel.NewContext(bitcoinkernel.ChainTypeMainnet)
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

const (
	numPrepWorkers = 12 // number of parallel block preparation workers

	// more backlog or workers do not really improve over all sync speed
	maxBlockBacklog = numPrepWorkers * 3 // maximum number of blocks that can be in preparation/computation

)

// SyncWithAsyncBlockPreparationAndComputation has async block preparation and block computation
func SyncWithAsyncBlockPreparationAndComputation(
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

	cctx := ctx
	// cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	// defer cancel()

	blockIndex, err := chainman.GetBlockIndexFromHeight(int(startHeight))
	if err != nil {
		logging.L.Err(err).Msg("Failed to get block index from height")
		return err
	}

	// Create channels for block preparation and computation
	prepChan := make(chan *blockPrepResult, maxBlockBacklog)
	compChan := make(chan *blockCompResult, maxBlockBacklog*2) // might also just remove channel size limit
	errChan := make(chan error, 1)

	// Create a channel to coordinate block index access
	indexChan := make(chan *bitcoinkernel.BlockIndex, maxBlockBacklog*2)

	// Start block index feeder
	go func() {
		defer close(indexChan)
		for blockIndex != nil && (endHeight == nil || blockIndex.GetHeight() < *endHeight) {
			select {
			case <-cctx.Done():
				return
			case indexChan <- blockIndex:
				blockIndex = chainman.GetNextBlockIndex(blockIndex)
			}
		}
	}()

	// Start multiple block preparation workers
	var prepWg sync.WaitGroup
	prepWg.Add(numPrepWorkers)
	for w := 0; w < numPrepWorkers; w++ {
		go func(workerID int) {
			defer prepWg.Done()
			for blockIndex := range indexChan {
				select {
				case <-cctx.Done():
					return
				default:
					block, err := pullAndPrepareBlock(cctx, chainman, blockIndex)
					if err != nil {
						errChan <- fmt.Errorf("worker %d failed to prepare block %d: %w",
							workerID, blockIndex.GetHeight(), err)
						return
					}
					prepChan <- &blockPrepResult{
						block: block,
						index: blockIndex,
					}
				}
			}
		}(w)
	}

	// Start block computation worker
	var compWg sync.WaitGroup
	compWg.Add(1)
	go func() {
		defer compWg.Done()
		nextHeight := startHeight
		blockBuffer := make(map[uint32]*blockPrepResult)

		for result := range prepChan {
			select {
			case <-cctx.Done():
				return
			default:
				// Store the block in buffer if it's not the next one
				if result.block.GetBlockHeight() != nextHeight {
					blockBuffer[result.block.GetBlockHeight()] = result
					continue
				}

				// Process the next block
				logging.L.Trace().
					Int("prepChanSize", len(prepChan)).
					Int("compChanSize", len(compChan)).
					Msgf("Starting to process block %d", result.block.GetBlockHeight())

				err := indexer.HandleBlock(cctx, result.block)
				compChan <- &blockCompResult{
					height: result.block.GetBlockHeight(),
					err:    err,
				}
				nextHeight++

				// Process any buffered blocks that are now in sequence
				for {
					if buffered, ok := blockBuffer[nextHeight]; ok {
						err := indexer.HandleBlock(cctx, buffered.block)
						compChan <- &blockCompResult{
							height: buffered.block.GetBlockHeight(),
							err:    err,
						}
						delete(blockBuffer, nextHeight)
						nextHeight++
					} else {
						break
					}
				}

				logging.L.Info().Msgf("Finished processing block %d", result.block.GetBlockHeight())
			}
		}
		close(compChan)
	}()

	// Monitor results and handle errors
	go func() {
		prepWg.Wait()
		compWg.Wait()
		close(errChan)
	}()

	// Wait for completion or error
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
			return nil
		case <-cctx.Done():
			return cctx.Err()
		case result := <-compChan:
			if result.err != nil {
				logging.L.Err(result.err).Msgf("Failed to handle block %d", result.height)
			} else {
				logging.L.Info().
					Int("prepChanSize", len(prepChan)).
					Int("compChanSize", len(compChan)).
					Uint32("height", result.height).
					Msg("Successfully processed block")
			}
		}
	}
}

type blockPrepResult struct {
	block *KernelBlock
	index *bitcoinkernel.BlockIndex
}

type blockCompResult struct {
	height uint32
	err    error
}

func pullAndPrepareBlock(
	ctx context.Context,
	chainman *bitcoinkernel.ChainstateManager,
	blockIndex *bitcoinkernel.BlockIndex,
) (*KernelBlock, error) {
	logging.L.Debug().Msgf("Pulling and preparing block: %d", blockIndex.GetHeight())

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
		return nil, err
	}
	defer blockUndo.Close()

	block := NewKernelBlock(blockData, uint32(blockIndex.GetHeight()), blockUndo)
	if block == nil {
		err = errors.New("failed to create kernel block")
		logging.L.Err(err).Msg("Failed to create kernel block")
		return nil, err
	}

	_ = block.GetTransactions() // this will attach the transactions to the block

	return block, nil
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
