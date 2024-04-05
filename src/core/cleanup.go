package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"errors"
)

/*
todo needs a routine that cleans up the tweak data if no more utxos exist for a tx.
 The routine will then remove the corresponding tweak data from the db for every txid without light utxos.
 In an optimal implementation this should not be needed though.
*/

func removeSpentUTXOsAndTweaks(utxos []types.UTXO) error {
	// First delete the old UTXOs
	err := dblevel.DeleteBatchUTXOs(utxos)
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

/*
	err = dblevel.OverWriteTweaks(tweaksToOverwrite)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
*/
