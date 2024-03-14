package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"errors"
	"sort"
	"sync"
)

func SyncChain() error {
	common.InfoLogger.Println("starting sync")

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return err
	}
	common.InfoLogger.Printf("blockchain info: %+v\n", blockchainInfo)

	// Current logic is that we always just sync from where the user wants us to sync.
	// We won't sync below the height
	syncFromHeight := common.CatchUp

	// todo might need to change flow control to use break
	// number of headers that will maximally be fetched at once
	step := common.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []types.BlockHeader
		common.InfoLogger.Println("Getting next batch of headers from:", i)
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
			common.ErrorLogger.Println(err)
			return err
		}

		if len(heightsClean) == 0 {
			err = updateBlockchainInfo(blockchainInfo)
			if err != nil {
				common.WarningLogger.Println(err)
				return err
			}
			i += step
			continue
		}

		headers, err = GetBlockHeadersBatch(heightsClean)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
		sort.Slice(headers, func(i, j int) bool {
			return headers[i].Height < headers[j].Height
		})

		// todo needs to return error
		err = processHeaders(headers)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		i += step

		// this keeps the syncing process up to date with the chain tip
		// if syncing takes longer we avoid querying too many previous blocks in `HandleBlock`
		err = updateBlockchainInfo(blockchainInfo)
		if err != nil {
			common.WarningLogger.Println(err)
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
		common.ErrorLogger.Println(err)
		return err
	}
	if previousHeight < blockchainInfo.Blocks {
		common.InfoLogger.Println("increasing block height to:", blockchainInfo.Blocks)
	}
	return nil
}

// todo works most of the times but hangs sometimes.
//  Blocks are being skipped and not processed in the correct order.
//  We don't want that -> further debugging and robustness tests needed
func processHeaders(headers []types.BlockHeader) error {
	common.InfoLogger.Printf("Processing %d headers\n", len(headers))
	if len(headers) == 0 {
		common.WarningLogger.Println("No headers were passed")
		return nil
	}
	fetchedBlocks := make(chan *types.Block, common.MaxParallelRequests)
	semaphore := make(chan struct{}, common.MaxParallelRequests)

	var errG error
	var mu sync.Mutex // Mutex to protect shared resources

	// block fetcher routine
	go func() {
		for _, header := range headers {
			if errG != nil {
				common.ErrorLogger.Println(errG)
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
					if err.Error() == "block already processed" {
						// Log and skip this block since it's already been processed
						// send empty block to signal it was processed, will be skipped in processing loop
						fetchedBlocks <- &types.Block{Height: _header.Height}
						common.InfoLogger.Printf("Block %d already processed\n", _header.Height)
					} else {
						// For other errors, log and store the first occurrence, then exit
						common.ErrorLogger.Println(err)
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
	common.InfoLogger.Println("Syncing headers")

	headerInv, err := dblevel.FetchHighestBlockHeaderInv()
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		return err
	}

	var syncFromHeight uint32
	if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
		// we have to start before taproot activation height
		// some taproot style pubKeys exist since height ~614000 (the last height I checked)
		syncFromHeight = 500_000
	} else {
		// Sync from where the last header was set
		syncFromHeight = 500_000
		if syncFromHeight <= headerInv.Height {
			syncFromHeight = headerInv.Height + 1
		}
	}

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return err
	}
	common.InfoLogger.Printf("blockchain info: %+v\n", blockchainInfo)

	// todo might need to change flow control to use break
	// number of headers that will maximally be fetched at once
	step := common.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []types.BlockHeader
		common.InfoLogger.Println("Getting next batch of headers from:", i)

		var heights []uint32
		for height := i; height < i+step; height++ {
			heights = append(heights, height)
		}
		var heightsClean []uint32
		heightsClean, err = dblevel.GetMissingHeadersInv(heights)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		if len(heightsClean) == 0 {
			err = updateBlockchainInfo(blockchainInfo)
			if err != nil {
				common.WarningLogger.Println(err)
				return err
			}
			i += step
			continue
		}

		headers, err = GetBlockHeadersBatch(heightsClean)
		if err != nil {
			common.ErrorLogger.Println(err)
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
			common.ErrorLogger.Println(err)
			return err
		}

		// verify that headers are not already processed.
		// We pre-check this before it is checked in CheckBlock function, might be redundant

		i += step

		// this keeps the syncing process up to date with the chain tip
		// if syncing takes longer we avoid querying too many previous blocks in `HandleBlock`
		err = updateBlockchainInfo(blockchainInfo)
		if err != nil {
			common.WarningLogger.Println(err)
			return err
		}
	}
	return nil
}
