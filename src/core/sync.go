package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/mongodb"
	"sync"
)

func SyncChain() error {
	common.InfoLogger.Println("starting sync")

	//lastHeader, err := mongodb.RetrieveLastHeader()
	//if err != nil {
	//	// fatal due to startup condition
	//	common.ErrorLogger.Fatalln(err)
	//	return err
	//}

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return err
	}
	common.InfoLogger.Printf("blockchain info: %+v\n", blockchainInfo)

	// Current logic is that we always just sync from where the user wants us to sync.
	// We won't sync below the height
	syncFromHeight := common.CatchUp
	//if syncFromHeight > common.CatchUp {
	//	syncFromHeight = common.CatchUp
	//}

	// todo might need to change flow control to use break
	// how many headers are supposed to be fetched at once
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
		headers, err = GetBlockHeadersBatch(i, step) // todo verify the blocks always come in the correct order
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		// verify that headers are not already processed.
		// We pre-check this before it is checked in CheckBlock function, might be redundant
		var lastHeight *uint32
		lastHeight, err = mongodb.BulkCheckHeadersExist(headers)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		// todo only quick fix for tests
		//if lastHeight == nil {
		//	common.InfoLogger.Println("All headers were processed already. Skipping ahead...")
		//	err = updateBlockchainInfo(blockchainInfo)
		//	if err != nil {
		//		common.WarningLogger.Println(err)
		//		return err
		//	}
		//
		//	i += step
		//	continue
		//}

		// todo needs to return error
		err = processHeaders(headers, *lastHeight)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}

		// Increment 'i' by 'step' after processing the headers
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
func processHeaders(headers []types.BlockHeader, lastHeight uint32) error {
	common.InfoLogger.Printf("Processing %d headers\n", len(headers))
	if len(headers) == 0 {
		common.InfoLogger.Println("No headers were passed")
		return nil
	}
	fetchedBlocks := make(chan *types.Block, common.MaxParallelRequests)
	semaphore := make(chan struct{}, common.MaxParallelRequests)

	var errG error
	var mu sync.Mutex // Mutex to protect shared resources

	// block fetcher routine
	go func() {
		for _, header := range headers {
			if lastHeight != 0 && header.Height < lastHeight {
				// Skip already processed headers
				// last height is the first block that has to be processed
				continue
			}
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
						//common.InfoLogger.Printf("Block %d already processed\n", _header.Height)
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
	// Process block headers in order
	// todo only quick fix for debug
	if lastHeight != 0 {
		nextExpectedBlock = lastHeight
	} else {
		nextExpectedBlock = headers[0].Height
	}

	outOfOrderBlocks := make(map[uint32]*types.Block)

	// block processor
	for {
		if errG != nil {
			close(fetchedBlocks) // Close the channel to terminate the fetching goroutine
			return errG
		}
		// Exit condition: Stop when the expected next block is beyond the end block
		if nextExpectedBlock > headers[len(headers)-1].Height {
			close(fetchedBlocks) // Close the channel to terminate the fetching goroutine
			return errG
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
					nextExpectedBlock++
					continue
				}
				// Process block using its hash
				CheckBlock(block)
				nextExpectedBlock++
			}

			var ok = true
			for ok {
				if block, ok = outOfOrderBlocks[nextExpectedBlock]; ok {
					if block.Hash == "" {
						delete(outOfOrderBlocks, nextExpectedBlock)
						nextExpectedBlock++
						continue
					}
					CheckBlock(block)
					delete(outOfOrderBlocks, nextExpectedBlock)
					// Update next expected block
					nextExpectedBlock++
				}
			}

		}
	}
}
