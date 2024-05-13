package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"errors"
)

func overwriteUTXOsWithLookUp(utxos []types.UTXO) error {
	var utxosToOverwrite []types.UTXO

	for _, utxo := range utxos {
		fetchedUTXOs, err := dblevel.FetchByBlockHashAndTxidUTXOs(utxo.BlockHash, utxo.Txid)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			return err
		} else if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			// we skip if no entry was found. We don't want to insert those
			continue
		}
		// we actually don't have to check the fetched UTXOs. if any utxos were found for this transaction it means that it was eligible.
		// hence all taproot utxos have to be present
		_ = fetchedUTXOs
		utxosToOverwrite = append(utxosToOverwrite, utxo)
	}
	err := dblevel.InsertUTXOs(utxosToOverwrite)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return err
}

// todo construct the subsequent deletion of all utxos per transaction once all per transaction are spent
func markSpentUTXOsAndTweaks(utxos []types.UTXO) error {
	if len(utxos) == 0 {
		if common.Chain == common.Mainnet {
			// no warnings on other chains as it is very likely to not have any taproot outputs for several blocks on end
			common.WarningLogger.Println("no utxos to mark as spent")
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
		common.ErrorLogger.Println(err)
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
			common.ErrorLogger.Println(err)
			return err
		} else if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			//	if no UTXOs are left for a certain blockHash-txid combination we can remove the tweak
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
			common.ErrorLogger.Println(err)
			return err
		}
		if newBiggest != nil {
			// find the biggest UTXO for the tx and overwrite if necessary
			tweaksToOverwrite = append(tweaksToOverwrite, types.Tweak{
				BlockHash:    utxo.BlockHash,
				BlockHeight:  0,
				Txid:         utxo.Txid,
				Data:         [33]byte{},
				HighestValue: *newBiggest,
			})
		}
	}

	err = dblevel.DeleteBatchTweaks(tweaksToDelete)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	err = dblevel.OverWriteTweaks(tweaksToOverwrite)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return err
}

// ReindexDustLimitsOnly this routine adds the dust limit data to tweaks after a sync
func ReindexDustLimitsOnly() error {
	common.InfoLogger.Println("Reindexing dust limit from synced data")
	err := dblevel.DustOverwriteRoutine()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	common.InfoLogger.Println("Reindexing dust limit done")
	return nil

}

// Prune
// This function checks the utxo set and deletes all utxos of a transaction where all utxos are marked as spent.
// it can then also remove the tweak of completely spent transaction
func Prune() error {
	return nil
}
