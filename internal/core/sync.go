package core

import (
	"errors"
	"sort"
	"sync"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func SyncChain() error {
	logging.L.Info().Msg("starting sync")

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		logging.L.Err(err).Msg("error getting blockchain info")
		return err
	}
	logging.L.Info().Msgf("blockchain info: %+v\n", blockchainInfo)

	// Current logic is that we always just sync from where the user wants us to sync.
	// We won't sync below the height
	syncFromHeight := config.SyncStartHeight

	// todo might need to change flow control to use break
	// number of headers that will maximally be fetched at once
	step := config.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []types.BlockHeader
		logging.L.Info().Msgf("Getting next batch of headers from: %d", i)
		// todo find a way to skip ahead to the next unprocessed block from the catch up point.
		//  Maybe iterate over db before querying. Can either do before every query or
		//  once to get to a decent height to continue from. Anyways it should not be the case
		//  that we have a patchy history of processed blocks

		var heights []uint32
		for height := i; height < i+step; height++ {
			heights = append(heights, height)
		}
		var heightsClean []uint32
		heightsClean, err = dblevel.GetMissingHeadersInvFlag(heights, false) // only find unprocessed blocks heights
		if err != nil {
			logging.L.Err(err).Msg("error getting missing headers inv flag")
			return err
		}

		if len(heightsClean) == 0 {
			err = updateBlockchainInfo(blockchainInfo)
			if err != nil {
				logging.L.Warn().Err(err).Msg("error updating blockchain info")
				return err
			}
			i += step
			continue
		}

		headers, err = GetBlockHeadersBatch(heightsClean)
		if err != nil {
			logging.L.Err(err).Msg("error getting block headers batch")
			return err
		}
		sort.Slice(headers, func(i, j int) bool {
			return headers[i].Height < headers[j].Height
		})

		err = processHeaders(headers)
		if err != nil {
			logging.L.Err(err).Msg("error processing headers")
			return err
		}

		i += step

		// this keeps the syncing process up to date with the chain tip
		// if syncing takes longer we avoid querying too many previous blocks in `HandleBlock`
		err = updateBlockchainInfo(blockchainInfo)
		if err != nil {
			logging.L.Warn().Err(err).Msg("error updating blockchain info")
			return err
		}
	}
	return nil
}

func updateBlockchainInfo(blockchainInfo *types.BlockchainInfo) error {
	var err error
	previousHeight := blockchainInfo.Blocks
	blockchainInfo, err = GetBlockchainInfo()
	if err != nil {
		logging.L.Err(err).Msg("error getting blockchain info")
		return err
	}
	if previousHeight < blockchainInfo.Blocks {
		logging.L.Info().Msgf("increasing block height to: %d", blockchainInfo.Blocks)
	}
	return nil
}

func processHeaders(headers []types.BlockHeader) error {
	logging.L.Info().Msgf("Processing %d headers\n", len(headers))
	if len(headers) == 0 {
		logging.L.Warn().Msg("No headers were passed")
		return nil
	}
	fetchedBlocks := make(chan *types.Block, config.MaxParallelRequests)
	semaphore := make(chan struct{}, config.MaxParallelRequests)

	var errG error
	var mu sync.Mutex // Mutex to protect shared resources

	// block fetcher routine
	go func() {
		for _, header := range headers {
			if errG != nil {
				logging.L.Err(errG).Msg("error processing headers")
				break // If an error occurred, break the loop
			}

			semaphore <- struct{}{} // Acquire a slot
			go func(_header types.BlockHeader) {
				//start := time.Now()
				defer func() {
					<-semaphore // Release the slot
				}()

				block, err := PullBlock(_header.Hash)
				if err != nil {
					if err.Error() == "block already processed" { // todo built in error
						// Log and skip this block since it's already been processed
						// send empty block to signal it was processed, will be skipped in processing loop
						fetchedBlocks <- &types.Block{Height: _header.Height}
						logging.L.Info().Msgf("Block %d already processed\n", _header.Height)
					} else {
						// For other errors, log and store the first occurrence, then exit
						logging.L.Err(err).Msg("error processing headers")
						mu.Lock()
						if errG == nil {
							errG = err // Store the first error that occurs
						}
						mu.Unlock()
						return
					}
				} else {
					fetchedBlocks <- block // Send the fetched block to the channel
				}
				//common.InfoLogger.Printf("It took %d ms to pull block %d\n", time.Now().Sub(start).Milliseconds(), _header.Height)
			}(header)
		}
	}()

	var nextExpectedBlock uint32

	var nextExpectedBlockMap = map[uint32]uint32{}
	for i := 0; i < len(headers)-1; i++ {
		nextExpectedBlockMap[headers[i].Height] = headers[i+1].Height
	}
	nextExpectedBlock = headers[0].Height

	outOfOrderBlocks := make(map[uint32]*types.Block)

	// block processor
	for {
		if errG != nil {
			close(fetchedBlocks) // Close the channel to terminate the fetching goroutine
			return errG
		}
		// todo if last element is processed it should always be 0
		//// Exit condition: Stop when the expected next block is beyond the end block
		//if nextExpectedBlock > headers[len(headers)-1].Height {
		//	close(fetchedBlocks) // Close the channel to terminate the fetching goroutine
		//	return errG
		//}
		if nextExpectedBlock == 0 {
			break
		}
		select {
		case block := <-fetchedBlocks:
			//common.InfoLogger.Println("Got block:", block.Height)
			// check whether the block is a filler block with only the height
			if block.Height != nextExpectedBlock {
				// Temporarily store out-of-order block header
				outOfOrderBlocks[block.Height] = block
			} else {
				if block.Hash == "" {
					nextExpectedBlock = nextExpectedBlockMap[nextExpectedBlock]
				} else {
					// Process block using its hash
					CheckBlock(block)
					nextExpectedBlock = nextExpectedBlockMap[nextExpectedBlock]
				}
			}

			var ok = true
			for ok {
				if block, ok = outOfOrderBlocks[nextExpectedBlock]; ok {
					if block.Hash == "" {
						delete(outOfOrderBlocks, nextExpectedBlock)
						nextExpectedBlock = nextExpectedBlockMap[nextExpectedBlock]
						continue
					}
					CheckBlock(block)
					delete(outOfOrderBlocks, nextExpectedBlock)
					// Update next expected block
					nextExpectedBlock = nextExpectedBlockMap[nextExpectedBlock]
				}
			}
		}
	}
	return nil
}

func PreSyncHeaders() error {
	logging.L.Info().Msg("Syncing headers")

	headerInv, err := dblevel.FetchHighestBlockHeaderInv()
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
		logging.L.Err(err).Msg("error fetching highest block header inv")
		return err
	}

	var syncFromHeight uint32
	if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
		// we have to start before taproot activation height
		// some taproot style pubKeys exist since height ~614000 (the last height I checked)
		syncFromHeight = config.HeaderMustSyncHeight()
	} else {
		// Sync from where the last header was set
		syncFromHeight = config.HeaderMustSyncHeight()
		if syncFromHeight <= headerInv.Height {
			syncFromHeight = headerInv.Height + 1
		}
	}

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error getting blockchain info")
		return err
	}
	logging.L.Info().Any("blockchain_info", blockchainInfo).Msg("blockchain info")

	// todo might need to change flow control to use break
	// number of headers that will maximally be fetched at once
	step := config.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []types.BlockHeader
		logging.L.Info().Msgf("Getting next batch of headers from: %d", i)

		var heights []uint32
		for height := i; height < i+step; height++ {
			heights = append(heights, height)
		}
		var heightsClean []uint32
		heightsClean, err = dblevel.GetMissingHeadersInv(heights)
		if err != nil {
			logging.L.Err(err).Msg("error getting missing headers inv")
			return err
		}

		if len(heightsClean) == 0 {
			err = updateBlockchainInfo(blockchainInfo)
			if err != nil {
				logging.L.Warn().Err(err).Msg("error updating blockchain info")
				return err
			}
			i += step
			continue
		}

		headers, err = GetBlockHeadersBatch(heightsClean)
		if err != nil {
			logging.L.Err(err).Msg("error getting block headers batch")
			return err
		}
		sort.Slice(headers, func(i, j int) bool {
			return headers[i].Height < headers[j].Height
		})

		// convert BlockHeaders to BlockerHeadersInv
		var headersInv []types.BlockHeaderInv
		for _, header := range headers {
			headersInv = append(headersInv, types.BlockHeaderInv{
				Hash:   header.Hash,
				Height: header.Height,
				Flag:   false,
			})
		}
		err = dblevel.InsertBatchBlockHeaderInv(headersInv)
		if err != nil {
			logging.L.Err(err).Msg("error inserting batch block header inv")
			return err
		}

		// verify that headers are not already processed.
		// We pre-check this before it is checked in CheckBlock function, might be redundant

		i += step

		// this keeps the syncing process up to date with the chain tip
		// if syncing takes longer we avoid querying too many previous blocks in `HandleBlock`
		err = updateBlockchainInfo(blockchainInfo)
		if err != nil {
			logging.L.Warn().Err(err).Msg("error updating blockchain info")
			return err
		}
	}
	return nil
}
