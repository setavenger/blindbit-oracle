package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
)

func CreateLightUTXOs(block *types.Block) []types.UTXO {
	common.InfoLogger.Println("Getting new UTXOs")
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
		spentUTXOs = append(spentUTXOs, extractSpentTaprootPubKeysFromTx(&tx, block)...)
	}

	return spentUTXOs
}

func extractSpentTaprootPubKeysFromTx(tx *types.Transaction, block *types.Block) []types.UTXO {
	var spentUTXOs []types.UTXO

	for _, vin := range tx.Vin {
		if vin.Coinbase != "" {
			continue
		}
		switch vin.Prevout.ScriptPubKey.Type {

		case "witness_v1_taproot":
			// requires a pre-sync of height from taproot activation 709632 for blockHash mapping,
			// todo fails if CPFP prevout.height will be current block check for that
			var blockHash string
			if vin.Prevout.Height == block.Height {
				//	CPFP case prevout can be in the same block as the current height and
				//	hence will not be found in the HeadersInvDB/HeadersDB as the header is inserted last in the process
				blockHash = block.Hash
			} else {
				// after making sure we don't have prevout and vin in the same block we can do a standard lookup
				headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(vin.Prevout.Height)
				if err != nil {
					common.ErrorLogger.Println(err)
					// panic becuase if this fails it means we have incomplete data which requires a sync
					common.ErrorLogger.Printf("tx: %+v\n", tx)
					common.ErrorLogger.Println("Headers not synced from taproot activation height (709632). Either build complete index or fully sync headers only.")
					panic(err)
				}
				blockHash = headerInv.Hash
			}

			//832974
			spentUTXOs = append(spentUTXOs, types.UTXO{
				Txid:      vin.Txid,
				Vout:      vin.Vout,
				Value:     common.ConvertFloatBTCtoSats(vin.Prevout.Value),
				BlockHash: blockHash,
				Spent:     true,
			})
		default:
			continue
		}
	}

	return spentUTXOs
}
