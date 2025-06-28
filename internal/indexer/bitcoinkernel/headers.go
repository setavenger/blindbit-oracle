package bitcoinkernel

import (
	"log"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/setavenger/go-bitcoinkernel/pkg/bitcoinkernel"
)

// todo: might need a shared context or chainmanager to perform (better)
func IndexHeaders() error {
	logging.L.Info().Msg("Indexing headers")
	// selecting chain tip and going to genesis is easier than moving forwards in time
	chainman, err := bitcoinkernel.NewChainstateManager(kernelContext, datadir)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create chainstate manager")
		return err
	}
	defer chainman.Close()

	// Get the block index at the chain tip
	blockIndex, err := chainman.GetBlockIndexFromTip()
	if err != nil {
		log.Fatalf("Failed to get block index from tip: %v", err)
		return err
	}
	defer blockIndex.Close()

	// Get block height
	height := blockIndex.GetHeight()

	// its +1 because we need to include the tip
	// zero index is genesis
	headersInv := make([]types.BlockHeaderInv, height+1)

	// make a slice of all headers from genesis to tip
	idxCounter := 0
	for ; blockIndex != nil; blockIndex = blockIndex.Prev() {
		headersInv[idxCounter] = types.BlockHeaderInv{
			Hash:   *blockIndex.GetBlockHash().GetBytes(),
			Height: blockIndex.GetHeight(),
			Flag:   false,
		}
		idxCounter++
		if idxCounter%25_000 == 0 {
			logging.L.Info().Msgf("Indexed %d headers", idxCounter)
		}
	}

	err = dblevel.InsertBatchBlockHeaderInv(headersInv)
	if err != nil {
		logging.L.Err(err).Msg("error inserting batch block header inv")
		return err
	}

	return nil
}
