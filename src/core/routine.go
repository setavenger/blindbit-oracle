package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"time"
)

func CheckForNewBlockRoutine() {
	for true {
		select {
		case <-time.NewTicker(5 * time.Second).C:
			hash, err := GetBestBlockHash()
			if err != nil {
				common.ErrorLogger.Println(err)
			}
			checkBlockHash(hash)
		}
	}
}

// checkBlockHash checks whether the blockchain tip has already been processed.
func checkBlockHash(blockHash string) {
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
		common.DebugLogger.Println(blockHash, "could not be handled")
		common.ErrorLogger.Fatalln(err)
		return
	}

	// todo maybe optimize for single insertion
	mongodb.SaveBulkHeaders([]*common.BlockHeader{{
		Hash:          block.Hash,
		PrevBlockHash: block.PreviousBlockHash,
		Timestamp:     block.Timestamp,
		Height:        block.Height,
	}})

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
	if block.Height > common.MinHeightToProcess {
		checkBlockHash(block.PreviousBlockHash)
	}

	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	// get spent taproot UTXOs
	taprootSpent := extractSpentTaprootPubKeysFromBlock(block)
	for _, spentUTXO := range taprootSpent {
		common.DebugLogger.Printf("Deleting Output: %s:%d\n", spentUTXO.Txid, spentUTXO.Vout)
		err = mongodb.DeleteLightUTXOByTxIndex(spentUTXO.Txid, spentUTXO.Vout)
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		mongodb.SaveSpentUTXO(&spentUTXO)
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
	mongodb.SaveFilterTaproot(&common.Filter{
		FilterType:  4,
		BlockHeight: block.Height,
		Data:        cFilterTaproot,
		BlockHeader: block.Hash,
	})

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

	return nil, err
}
