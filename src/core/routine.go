package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"errors"
	"fmt"
	"time"
)

func CheckForNewBlockRoutine() {
	common.InfoLogger.Println("starting check_for_new_block_routine")
	for true {
		select {
		case <-time.NewTicker(5 * time.Second).C:
			blockHash, err := GetBestBlockHash()
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			err = fullProcessBlockHash(blockHash)
			if err != nil {
				common.ErrorLogger.Println(err)
				return
			}
		}
	}
}

func fullProcessBlockHash(blockHash string) error {
	block, err := PullBlock(blockHash)
	if err != nil && err.Error() != "block already processed" {
		common.ErrorLogger.Println(err)
		return err
	}
	if block == nil {
		return nil
	}
	// check whether previous block has already been processed
	// we do the check before so that we can subsequently delete spent UTXOs
	// this should not be a problem and only apply in very few cases
	// the index should be caught up on startup and hence a previous block
	// will most likely only be squeezed in if there were several blocks in between tip queries
	if block.Height > common.CatchUp {
		err = fullProcessBlockHash(block.PreviousBlockHash)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
	}

	CheckBlock(block)
	return err
}

func PullBlock(blockHash string) (*common.Block, error) {
	if len(blockHash) != 64 {
		common.ErrorLogger.Println("block_hash invalid", blockHash)
		return nil, errors.New(fmt.Sprintf("block_hash invalid: %s", blockHash))
	}
	// this method is preferred over lastHeader because then this function can be called for PreviousBlockHash
	found, err := mongodb.CheckHeaderExists(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	if found {
		common.DebugLogger.Printf("Block: %s has already been processed\n", blockHash)
		// if we already processed the header into our DB don't do anything
		return nil, errors.New("block already processed")
	}
	block, err := GetFullBlockPerBlockHash(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	common.InfoLogger.Println("Received block:", blockHash)
	return block, nil
}

// CheckBlock checks whether the block hash has already been processed and will process the block if needed
func CheckBlock(block *common.Block) {
	// todo this should fail at the highest instance were its wrapped in,
	//  fatal made sense here while it only had one use,
	//  but might not want to exit the program if used in other locations

	err := HandleBlock(block)
	if err != nil {
		// todo handle better more gracefully, maybe retries
		common.DebugLogger.Println("failed for block:", block.Hash)
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
	common.InfoLogger.Println("successfully processed block:", block.Height)
	if err != nil {
		common.DebugLogger.Println("could not insert header for:", block.Hash)
		return
	}
}

func HandleBlock(block *common.Block) error {
	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	// get spent taproot UTXOs
	taprootSpent := extractSpentTaprootPubKeysFromBlock(block)

	err := mongodb.DeleteLightUTXOsBatch(taprootSpent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// save spent utxos to db
	common.InfoLogger.Println("Saving Spent UTXOs")
	err = mongodb.BulkInsertSpentUTXOs(taprootSpent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// build light UTXOs
	common.InfoLogger.Println("Processing Light UTXOs")
	lightUTXOs := CreateLightUTXOs(block)
	common.InfoLogger.Println("Light UTXOs processed")
	err = mongodb.BulkInsertLightUTXOs(lightUTXOs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// create special block filter
	cFilterTaproot := BuildTaprootOnlyFilter(block)
	err = mongodb.SaveFilterTaproot(&common.Filter{
		FilterType:  4,
		BlockHeight: block.Height,
		Data:        cFilterTaproot,
		BlockHash:   block.Hash,
	})
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	tweaksForBlock, err := ComputeTweaksForBlock(block)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	err = mongodb.BulkInsertTweaks(tweaksForBlock)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return err
}
