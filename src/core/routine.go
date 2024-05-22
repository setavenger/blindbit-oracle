package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"errors"
	"fmt"
	"time"
)

func CheckForNewBlockRoutine() {
	common.InfoLogger.Println("starting check_for_new_block_routine")
	for {
		<-time.NewTicker(3 * time.Second).C
		blockHash, err := GetBestBlockHash()
		if err != nil {
			common.ErrorLogger.Println(err)
			// todo fail or restart after too many fails?
			continue
		}
		err = FullProcessBlockHash(blockHash)
		if err != nil {
			common.ErrorLogger.Println(err)
			return
		}
	}
}

func FullProcessBlockHash(blockHash string) error {
	block, err := PullBlock(blockHash)
	if err != nil && err.Error() != "block already processed" { // todo built in error
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
	if block.Height > common.SyncStartHeight {
		err = FullProcessBlockHash(block.PreviousBlockHash)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
	}

	CheckBlock(block)
	return err
}

func PullBlock(blockHash string) (*types.Block, error) {
	if len(blockHash) != 64 {
		common.ErrorLogger.Println("block_hash invalid:", blockHash)
		return nil, fmt.Errorf("block_hash invalid: %s", blockHash)
	}
	// this method is preferred over lastHeader because then this function can be called for PreviousBlockHash
	header, err := dblevel.FetchByBlockHashBlockHeader(blockHash)
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
		// we ignore no entry error
		common.ErrorLogger.Println(err)
		return nil, err
	}

	if header != nil {
		// todo might not want to constantly log this
		// common.DebugLogger.Printf("Block: %s has already been processed\n", blockHash)
		// if we already processed the header into our DB don't do anything
		return nil, errors.New("block already processed")
	}

	block, err := GetFullBlockPerBlockHash(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	//common.InfoLogger.Println("Received block:", blockHash)
	return block, nil
}

// CheckBlock checks whether the block hash has already been processed and will process the block if needed
func CheckBlock(block *types.Block) {
	// todo this should fail at the highest instance were its wrapped in,
	//  fatal made sense here while it only had one use,
	//  but might not want to exit the program if used in other locations
	//common.InfoLogger.Println("Processing block:", block.Height)

	err := HandleBlock(block)
	if err != nil {
		// todo handle better more gracefully, maybe retries
		common.DebugLogger.Println("failed for block:", block.Hash)
		// program should exit here because it means we are missing a block and this needs immediate attention
		common.ErrorLogger.Fatalln(err)
		return
	}

	// insert flagged BlockHeader last as that is the basis on which we pull new blocks
	err = dblevel.InsertBlockHeader(types.BlockHeader{
		Hash:          block.Hash,
		PrevBlockHash: block.PreviousBlockHash,
		Timestamp:     block.Timestamp,
		Height:        block.Height,
	})
	if err != nil {
		common.DebugLogger.Println("could not insert header for:", block.Hash)
		return
	}

	err = dblevel.InsertBlockHeaderInv(types.BlockHeaderInv{
		Hash:   block.Hash,
		Height: block.Height,
		Flag:   true,
	})
	if err != nil {
		common.DebugLogger.Println("could not insert inverted header for:", block.Height, block.Hash)
		return
	}

	common.InfoLogger.Println("successfully processed block:", block.Height)

}

func HandleBlock(block *types.Block) error {
	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	common.DebugLogger.Println("Computing tweaks...")
	tweaksForBlock, err := ComputeTweaksForBlock(block)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.DebugLogger.Println("Tweaks computed...")

	if common.TweakIndexFullNoDust || common.TweakIndexFullIncludingDust {
		// build map for sorting
		tweaksForBlockMap := map[string]types.Tweak{}
		for _, tweak := range tweaksForBlock {
			tweaksForBlockMap[tweak.Txid] = tweak
		}

		// we only create one of the two filters no dust can be derived from dust but not vice versa
		// So we build the dust index if dust is needed and no-dust if off but not both
		if common.TweakIndexFullIncludingDust {
			// full index with dust filter possibility
			// todo should we sort, overhead created
			tweakIndexDust := types.TweakIndexDustFromTweakArray(tweaksForBlockMap, block)
			tweakIndexDust.BlockHash = block.Hash
			tweakIndexDust.BlockHeight = block.Height

			err = dblevel.InsertTweakIndexDust(tweakIndexDust)
			if err != nil {
				common.ErrorLogger.Println(err)
				return err
			}
		} else {
			// normal full index no dust
			// todo should we sort, overhead created
			tweakIndex := types.TweakIndexFromTweakArray(tweaksForBlockMap, block)
			tweakIndex.BlockHash = block.Hash
			tweakIndex.BlockHeight = block.Height
			err = dblevel.InsertTweakIndex(tweakIndex)
			if err != nil {
				common.ErrorLogger.Println(err)
				return err
			}
		}
	}

	if common.TweaksCutThroughWithDust {
		err = dblevel.InsertBatchTweaks(tweaksForBlock)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
	}

	// if we only want to generate the tweaks we exit here
	if common.TweaksOnly {
		return nil
	}

	// mark all transaction which have eligible outputs
	eligibleTransaction := map[string]struct{}{}
	for _, tweak := range tweaksForBlock {
		eligibleTransaction[tweak.Txid] = struct{}{}
	}

	// first we need to get the new outputs because some of them might/will be spent in the same block
	// build light UTXOs
	newUTXOs := ExtractNewUTXOs(block, eligibleTransaction)
	err = dblevel.InsertUTXOs(newUTXOs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// get spent taproot UTXOs
	taprootSpent := extractSpentTaprootPubKeysFromBlock(block)

	//err = removeSpentUTXOsAndTweaks(taprootSpent)
	// this will overwrite new UTXOs which were spent in the same block
	err = markSpentUTXOsAndTweaks(taprootSpent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// create special block filter
	cFilterNewUTXOs, err := BuildNewUTXOsFilter(block)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	//
	err = dblevel.InsertNewUTXOsFilter(cFilterNewUTXOs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	spentOutpointsIndex, err := BuildSpentUTXOIndex(taprootSpent, block)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = dblevel.InsertSpentOutpointsIndex(&spentOutpointsIndex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	cFilterSpentUTXOs, err := BuildSpentUTXOsFilter(spentOutpointsIndex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = dblevel.InsertSpentOutpointsFilter(cFilterSpentUTXOs)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return err
}
