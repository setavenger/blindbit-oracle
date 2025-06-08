package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func CheckForNewBlockRoutine() {
	logging.L.Info().Msg("starting check_for_new_block_routine")
	for {
		<-time.NewTicker(3 * time.Second).C
		blockHash, err := GetBestBlockHash()
		if err != nil {
			logging.L.Err(err).Msg("error getting best block hash")
			// todo fail or restart after too many fails?
			continue
		}
		err = FullProcessBlockHash(blockHash)
		if err != nil {
			logging.L.Err(err).Msg("error processing block")
			return
		}
	}
}

func FullProcessBlockHash(blockHash string) error {
	block, err := PullBlock(blockHash)
	if err != nil && err.Error() != "block already processed" { // todo built in error
		logging.L.Err(err).Msg("error pulling block from node")
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
	if block.Height > config.SyncStartHeight {
		err = FullProcessBlockHash(block.PreviousBlockHash)
		if err != nil {
			logging.L.Err(err).Msg("error processing previous block")
			return err
		}
	}

	CheckBlock(block)
	return err
}

func PullBlock(blockHash string) (*types.Block, error) {
	if len(blockHash) != 64 {
		logging.L.Err(fmt.Errorf("block_hash invalid: %s", blockHash)).Msg("block_hash invalid")
		return nil, fmt.Errorf("block_hash invalid: %s", blockHash)
	}
	// this method is preferred over lastHeader because then this function can be called for PreviousBlockHash
	header, err := dblevel.FetchByBlockHashBlockHeader(blockHash)
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
		// we ignore no entry error
		logging.L.Err(err).Msg("error fetching block header")
		return nil, err
	}

	if header != nil {
		// if we already processed the header into our DB don't do anything
		return nil, errors.New("block already processed")
	}

	block, err := GetFullBlockPerBlockHash(blockHash)
	if err != nil {
		logging.L.Err(err).Msg("error getting full block per block hash")
		return nil, err
	}
	return block, nil
}

// CheckBlock checks whether the block hash has already been processed and will process the block if needed
func CheckBlock(block *types.Block) {
	// todo add return type error
	// todo this should fail at the highest instance were its wrapped in,
	//  fatal made sense here while it only had one use,
	//  but might not want to exit the program if used in other locations

	// logging.L.Info().Msgf("Processing block: %d", block.Height)
	logging.L.Trace().Msgf("block: %d", block.Height)

	err := HandleBlock(block)
	if err != nil {
		// todo handle better more gracefully, maybe retries
		logging.L.Err(err).Msgf("failed for block: %s", block.Hash)
		// program should exit here because it means we are missing a block and this needs immediate attention
		logging.L.Fatal().Err(err).Msgf("failed for block: %s", block.Hash)
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
		logging.L.Err(err).
			Str("blockhash", block.Hash).
			Msgf("could not insert header for: %s", block.Hash)

		return
	}

	err = dblevel.InsertBlockHeaderInv(types.BlockHeaderInv{
		Hash:   block.Hash,
		Height: block.Height,
		Flag:   true,
	})
	if err != nil {
		logging.L.Err(err).
			Uint32("height", block.Height).
			Str("blockhash", block.Hash).
			Msg("could not insert inverted header for")
		return
	}

	logging.L.Info().
		Uint32("height", block.Height).
		Str("blockhash", block.Hash).
		Msg("successfully processed block")

}

func HandleBlock(block *types.Block) error {
	// todo the next sections can potentially be optimized by combining them into one loop where
	//  all things are extracted from the blocks transaction data

	logging.L.Debug().Msg("Computing tweaks...")
	tweaksForBlock, err := ComputeTweaksForBlock(block)
	if err != nil {
		logging.L.Err(err).Msg("error computing tweaks")
		return err
	}
	logging.L.Debug().Msg("Tweaks computed...")

	if config.TweakIndexFullNoDust || config.TweakIndexFullIncludingDust {
		// build map for sorting
		tweaksForBlockMap := map[string]types.Tweak{}
		for _, tweak := range tweaksForBlock {
			tweaksForBlockMap[tweak.Txid] = tweak
		}

		// we only create one of the two filters no dust can be derived from dust but not vice versa
		// So we build the dust index if dust is needed and no-dust if off but not both
		if config.TweakIndexFullIncludingDust {
			// full index with dust filter possibility
			// todo should we sort, overhead created
			tweakIndexDust := types.TweakIndexDustFromTweakArray(tweaksForBlockMap, block)
			tweakIndexDust.BlockHash = block.Hash
			tweakIndexDust.BlockHeight = block.Height

			err = dblevel.InsertTweakIndexDust(tweakIndexDust)
			if err != nil {
				logging.L.Err(err).Msg("error inserting tweak index dust")
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
				logging.L.Err(err).Msg("error inserting tweak index")
				return err
			}
		}
	}

	if config.TweaksCutThroughWithDust {
		err = dblevel.InsertBatchTweaks(tweaksForBlock)
		if err != nil {
			logging.L.Err(err).Msg("error inserting batch tweaks")
			return err
		}
	}

	// if we only want to generate the tweaks we exit here
	if config.TweaksOnly {
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
		logging.L.Err(err).Msg("error inserting utxos")
		return err
	}

	// get spent taproot UTXOs
	taprootSpent := extractSpentTaprootPubKeysFromBlock(block)

	//err = removeSpentUTXOsAndTweaks(taprootSpent)
	// this will overwrite new UTXOs which were spent in the same block
	err = markSpentUTXOsAndTweaks(taprootSpent)
	if err != nil {
		logging.L.Err(err).Msg("error marking spent utxos and tweaks")
		return err
	}

	// create special block filter
	cFilterNewUTXOs, err := BuildNewUTXOsFilter(block)
	if err != nil {
		logging.L.Err(err).Msg("error building new utxos filter")
		return err
	}

	//
	err = dblevel.InsertNewUTXOsFilter(cFilterNewUTXOs)
	if err != nil {
		logging.L.Err(err).Msg("error inserting new utxos filter")
		return err
	}

	spentOutpointsIndex, err := BuildSpentUTXOIndex(taprootSpent, block)
	if err != nil {
		logging.L.Err(err).Msg("error building spent utxo index")
		return err
	}

	err = dblevel.InsertSpentOutpointsIndex(&spentOutpointsIndex)
	if err != nil {
		logging.L.Err(err).Msg("error inserting spent utxo index")
		return err
	}

	cFilterSpentUTXOs, err := BuildSpentUTXOsFilter(spentOutpointsIndex)
	if err != nil {
		logging.L.Err(err).Msg("error building spent utxos filter")
		return err
	}

	err = dblevel.InsertSpentOutpointsFilter(cFilterSpentUTXOs)
	if err != nil {
		logging.L.Err(err).Msg("error inserting spent utxos filter")
		return err
	}

	return err
}
