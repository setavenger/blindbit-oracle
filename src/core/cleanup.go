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

func removeSpentUTXOs(utxos []types.UTXO) error {
	err := dblevel.DeleteBatchUTXOs(utxos)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

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
	for _, utxo := range cleanUTXOs {
		_, err := dblevel.FetchByBlockHashAndTxidUTXOs(utxo.BlockHash, utxo.Txid)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			return err
		} else if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			//	if no UTXOs are left for a certain blockHash txid combination we can remove the tweak
			tweaksToDelete = append(tweaksToDelete, types.Tweak{
				// we only need those Fields to serialise the key
				BlockHash: utxo.BlockHash,
				Txid:      utxo.Txid,
			})
		}
	}

	err = dblevel.DeleteBatchTweaks(tweaksToDelete)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	// this is already logged in the DB.Delete function
	//common.InfoLogger.Printf("Deleted %d UTXOs\n", len(utxos))

	return err
}
