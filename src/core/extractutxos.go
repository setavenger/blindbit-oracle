package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
)

func CreateLightUTXOs(block *types.Block) []types.UTXO {
	var utxos []types.UTXO
	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				utxos = append(utxos, types.UTXO{
					Txid:         tx.Txid,
					Vout:         vout.N,
					Value:        common.ConvertFloatBTCtoSats(vout.Value),
					ScriptPubKey: vout.ScriptPubKey.Hex,
					BlockHeight:  block.Height,
					BlockHash:    block.Hash,
					Timestamp:    block.Timestamp,
				})
			}
		}
	}

	return utxos
}

func extractSpentTaprootPubKeysFromBlock(block *types.Block) []types.UTXO {
	var spentUTXOs []types.UTXO
	for _, tx := range block.Txs {
		spentUTXOs = append(spentUTXOs, extractSpentTaprootPubKeysFromTx(&tx)...)
	}

	return spentUTXOs
}

func extractSpentTaprootPubKeysFromTx(tx *types.Transaction) []types.UTXO {
	var spentUTXOs []types.UTXO

	for _, vin := range tx.Vin {
		if vin.Coinbase != "" {
			continue
		}
		switch vin.Prevout.ScriptPubKey.Type {

		case "witness_v1_taproot":
			// requires a pre-sync of height from taproot activation 709632 for blockHash mapping,
			headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(vin.Prevout.Height)
			if err != nil {
				common.ErrorLogger.Println(err)
				// panic becuase if this fails it means we have incomplete data which requires a sync
				common.ErrorLogger.Println("Headers not synced from taproot activation height (709632). Either build complete index or fully sync headers only.")
				panic(err)
			}

			spentUTXOs = append(spentUTXOs, types.UTXO{
				Txid:      vin.Txid,
				Vout:      vin.Vout,
				Value:     common.ConvertFloatBTCtoSats(vin.Prevout.Value),
				BlockHash: headerInv.Hash,
				Spent:     true,
			})
		default:
			continue
		}
	}

	return spentUTXOs
}
