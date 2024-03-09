package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
)

func SyncChain() error {
	common.InfoLogger.Println("starting sync")

	lastHeader, err := mongodb.RetrieveLastHeader()
	if err != nil {
		// fatal due to startup condition
		common.ErrorLogger.Fatalln(err)
		return err
	}

	blockchainInfo, err := GetBlockchainInfo()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return err
	}
	common.InfoLogger.Printf("blockchain info: %+v\n", blockchainInfo)

	syncFromHeight := lastHeader.Height
	if syncFromHeight > common.CatchUp {
		syncFromHeight = common.CatchUp
	}

	// todo might need to change flow control to use break
	// how many headers are supposed to be fetched at once
	step := common.SyncHeadersMaxPerCall
	for i := syncFromHeight; i < blockchainInfo.Blocks; {
		// Adjust for the last run when there are fewer headers left than the step; avoids index out of range
		if i+step > blockchainInfo.Blocks {
			step = blockchainInfo.Blocks - i + 1 // needs to be +1 because GetBlockHeadersBatch starts at start height and is hence technically zero indexed
		}

		var headers []common.BlockHeader
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

		if lastHeight == nil {
			common.InfoLogger.Println("All headers were processed already. Skipping ahead...")
			err = updateBlockchainInfo(blockchainInfo)
			if err != nil {
				common.WarningLogger.Println(err)
				return err
			}

			i += step
			continue
		}

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

func updateBlockchainInfo(blockchainInfo *common.BlockchainInfo) error {
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
func processHeaders(headers []common.BlockHeader, lastHeight uint32) error {
	common.InfoLogger.Printf("Processing %d headers\n", len(headers))
	if len(headers) == 0 {
		common.InfoLogger.Println("No headers were passed")
		return nil
	}
	// Define a channel to receive fetched block headers
	fetchedBlocks := make(chan *common.Block, common.MaxParallelRequests)
	fetchedBlocksConf := make(chan struct{})
	// Create a buffered channel to control the number of concurrent goroutines
	semaphore := make(chan struct{}, common.MaxParallelRequests)
	nextExpectedBlock := headers[0].Height

	var errG error
	go func() {
		// Start fetching block headers in parallel
		for _, header := range headers {
			if header.Height <= lastHeight {
				// we skip all headers which have been processed already as determined by prior db check
				continue
			}
			semaphore <- struct{}{} // Acquire a slot

			go func(_header common.BlockHeader) {
				defer func() { <-semaphore }() // Release the slot

				block, err := PullBlock(_header.Hash)
				fetchedBlocksConf <- struct{}{} // always send in just to confirm pull
				if err != nil && err.Error() != "block already processed" {
					errG = err
					common.ErrorLogger.Println(err)
					return
				} else if err == nil {
					// todo make sure that there is no scenario where this logic chain breaks
					fetchedBlocks <- block
				} else if err.Error() == "block already processed" {
					if header.Height >= nextExpectedBlock {
						nextExpectedBlock = header.Height + 1
					}
				} else {
					common.DebugLogger.Println("err:", err)
					common.DebugLogger.Println("block:", block)
					common.WarningLogger.Println("check out this case")
				}
				return
			}(header)
			if errG != nil {
				common.DebugLogger.Println("Failed processing:", header.Hash)
				common.ErrorLogger.Println(errG)
				break
			}
		}
		// Wait for all goroutines to finish
		for i := 0; i < cap(semaphore); i++ {
			semaphore <- struct{}{}
		}
	}()

	// Define a map to temporarily hold out-of-order block headers keyed by height
	outOfOrderBlocks := make(map[uint32]*common.Block)

	// Process block headers in order
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
			if block.Height == nextExpectedBlock {
				// Process block using its hash
				CheckBlock(block)

				// Update next expected block
				if nextExpectedBlock < block.Height {
					nextExpectedBlock = block.Height + 1
				}
				nextExpectedBlock++
			} else {
				// Temporarily store out-of-order block header
				outOfOrderBlocks[block.Height] = block
			}
		case <-fetchedBlocksConf:
			if block, ok := outOfOrderBlocks[nextExpectedBlock]; ok {
				CheckBlock(block)
				delete(outOfOrderBlocks, nextExpectedBlock)
				// Update next expected block
				if nextExpectedBlock < block.Height {
					nextExpectedBlock = block.Height + 1
				}
			}
		}
	}
}
