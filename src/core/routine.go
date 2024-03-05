package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"time"
)

func CheckForNewBlockRoutine() {
	common.InfoLogger.Println("starting check_for_new_block_routine")
	for true {
		select {
		case <-time.NewTicker(5 * time.Second).C:
			hash, err := GetBestBlockHash()
			if err != nil {
				common.ErrorLogger.Println(err)
			}
			CheckBlockHash(hash)
		}
	}
}

// CheckBlockHash checks whether the block hash has already been processed and will process the block if needed
func CheckBlockHash(blockHash string) {
	// this method is preferred over lastHeader because then this function can be called for PreviousBlockHash
	found, err := mongodb.CheckHeaderExists(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return
	}
	if found {
		// if we already processed the header into our DB don't do anything
		return
	}

	block, err := HandleBlock(blockHash)
	if err != nil {
		common.DebugLogger.Println("failed for block:", blockHash)
		// program should exit here because it means we are missing a block and this needs immediate attention
		common.ErrorLogger.Fatalln(err)
		return
	}

	// todo maybe optimize with a single insertion
	err = mongodb.BulkInsertHeaders([]common.BlockHeader{{
		Hash:          block.Hash,
		PrevBlockHash: block.PreviousBlockHash,
		Timestamp:     block.Timestamp,
		Height:        block.Height,
	}})
	if err != nil {
		common.DebugLogger.Println(blockHash, "could not be handled")
		return
	}

}

func HandleBlock(blockHash string) (*common.Block, error) {
	block, err := GetFullBlockPerBlockHash(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// check whether previous block has already been processed
	// we do the check before so that we can subsequently delete spent UTXOs
	// this should not be a problem and only apply in very few cases
	// the index should be caught up on startup and hence a previous block
	// will most likely only be squeezed in if there were several blocks in between tip queries
	if block.Height > common.CatchUp {
		CheckBlockHash(block.PreviousBlockHash)
	}

	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	// get spent taproot UTXOs
	taprootSpent := extractSpentTaprootPubKeysFromBlock(block)

	err = mongodb.DeleteLightUTXOsBatch(taprootSpent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// save spent utxos to db
	err = mongodb.BulkInsertSpentUTXOs(taprootSpent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// build light UTXOs
	lightUTXOs := CreateLightUTXOs(block)
	err = mongodb.BulkInsertLightUTXOs(lightUTXOs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// create special block filter
	cFilterTaproot := BuildTaprootOnlyFilter(block)
	err = mongodb.SaveFilterTaproot(&common.Filter{
		FilterType:  4,
		BlockHeight: block.Height,
		Data:        cFilterTaproot,
		BlockHeader: block.Hash,
	})
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	tweaksForBlock, err := ComputeTweaksForBlock(block)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	err = mongodb.SaveTweakIndex(tweaksForBlock)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return block, err
}
