package core

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func overwriteUTXOsWithLookUp(utxos []types.UTXO) error {
	logging.L.Trace().Msg("overwriting utxos with lookup")
	var utxosToOverwrite []*types.UTXO

	for _, utxo := range utxos {
		_, err := dblevel.FetchByBlockHashAndTxidUTXOs(utxo.BlockHash, utxo.Txid)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			logging.L.Err(err).Msg("error fetching utxos")
			return err
		} else if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			// we skip if no entry was found. We don't want to insert those
			continue
		}
		// We actually don't have to check the fetched UTXOs.
		// If any utxos were found for this transaction it means that it was eligible.
		// Hence all taproot utxos have to be present
		utxosToOverwrite = append(utxosToOverwrite, &utxo)
	}
	err := dblevel.InsertUTXOs(utxosToOverwrite)
	alreadyCheckedTxids := make(map[string]struct{})
	for _, utxo := range utxosToOverwrite {
		if _, ok := alreadyCheckedTxids[utxo.Txid]; ok {
			continue
		}
		var key []byte
		key, err = utxo.SerialiseKey()
		if err != nil {
			logging.L.Err(err).Msg("error serialising utxo key")
			return err
		}

		err = dblevel.PruneUTXOs(key[:64])
		if err != nil {
			logging.L.Err(err).Msg("error pruning utxos")
			return err
		}
		alreadyCheckedTxids[utxo.Txid] = struct{}{}
	}
	if err != nil {
		logging.L.Err(err).Msg("error pruning utxos")
		return err
	}
	return err
}

// todo construct the subsequent deletion of all utxos per transaction once all per transaction are spent
func markSpentUTXOsAndTweaks(utxos []types.UTXO) error {
	logging.L.Trace().Msg("marking utxos")
	if len(utxos) == 0 {
		if config.Chain == config.Mainnet {
			// no warnings on other chains as it is very likely to not have any taproot outputs for several blocks on end
			logging.L.Trace().Msg("no utxos to mark as spent")
		}
		return nil
	}

	// we cannot just insert-override the utxos as we will insert non-eligible utxos which were not in the DB for a good reason to begin with
	// needs a check against existing tweaks before we can insert

	// todo how to avoid inserting non-eligible utxos
	// probably the best solution is to check every x blocks and remove all utxos which cannot be mapped to a tweak
	// or even better remove all utxos per a transaction where all utxos are spent

	// current implementation is to check at block sync whether an utxo should be overridden/inserted or not

	// First overwrite the spend UTXOs which now have the spent flag set
	err := overwriteUTXOsWithLookUp(utxos)
	if err != nil {
		logging.L.Err(err).Msg("error overwriting utxos with lookup")
		return err
	}

	// we can only delete old tweaks if we actually have the index available
	if !config.TweaksCutThroughWithDust {
		return err
	}

	// Now begin process to find the tweaks that need to be deleted

	// we only need to check for one utxo per txid, so we reduce the number of utxos -> fewer lookups in DB
	var cleanUTXOs []types.UTXO
	includedTxids := make(map[string]bool)

	for _, utxo := range utxos {
		if _, exists := includedTxids[utxo.Txid]; !exists {
			cleanUTXOs = append(cleanUTXOs, utxo)
			includedTxids[utxo.Txid] = true
		}
	}

	var tweaksToDelete []types.Tweak
	var tweaksToOverwrite []types.Tweak

	for _, utxo := range cleanUTXOs {
		var remainingUTXOs []types.UTXO
		remainingUTXOs, err = dblevel.FetchByBlockHashAndTxidUTXOs(utxo.BlockHash, utxo.Txid)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			// this is an actual error
			logging.L.Err(err).Msg("error fetching utxos")
			return err
		} else if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			// utxos can be already deleted at this point.
			// all utxos gone means we can remove the tweak
			tweaksToDelete = append(tweaksToDelete, types.Tweak{
				// we only need those Fields to serialise the key
				BlockHash: utxo.BlockHash,
				Txid:      utxo.Txid,
			})
			continue
		}
		var canBeRemoved = true
		for _, utxo := range remainingUTXOs {
			if !utxo.Spent {
				canBeRemoved = false
				break
			}
		}
		if canBeRemoved {
			tweaksToDelete = append(tweaksToDelete, types.Tweak{
				// we only need those Fields to serialise the key
				BlockHash: utxo.BlockHash,
				Txid:      utxo.Txid,
			})
			continue
		}

		var newBiggest *uint64
		newBiggest, err = types.FindBiggestRemainingUTXO(utxo, remainingUTXOs)
		if err != nil {
			logging.L.Err(err).Msg("error finding biggest remaining utxo")
			return err
		}
		if newBiggest != nil {
			// find the biggest UTXO for the tx and overwrite if necessary
			tweaksToOverwrite = append(tweaksToOverwrite, types.Tweak{
				BlockHash:    utxo.BlockHash,
				BlockHeight:  0,
				Txid:         utxo.Txid,
				TweakData:    [33]byte{},
				HighestValue: *newBiggest,
			})
		}
	}

	err = dblevel.DeleteBatchTweaks(tweaksToDelete)
	if err != nil {
		logging.L.Err(err).Msg("error deleting tweaks")
		return err
	}

	err = dblevel.OverWriteTweaks(tweaksToOverwrite)
	if err != nil {
		logging.L.Err(err).Msg("error overwriting tweaks")
		return err
	}

	return err
}

// ReindexDustLimitsOnly this routine adds the dust limit data to tweaks after a sync
func ReindexDustLimitsOnly() error {
	logging.L.Info().Msg("Reindexing dust limit from synced data")
	err := dblevel.DustOverwriteRoutine()
	if err != nil {
		logging.L.Err(err).Msg("error reindexing dust limit")
		return err
	}
	logging.L.Info().Msg("Reindexing dust limit done")
	return nil
}

// PruneUTXOs
// This function searches the UTXO set for transactions where all UTXOs are marked as spent, and removes those UTXOs.
func PruneAllUTXOs() error {
	logging.L.Info().Msg("Pruning All UTXOs")
	return dblevel.PruneUTXOs(nil)
}
