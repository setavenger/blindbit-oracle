package core

import (
	"SilentPaymentAppBackend/src/common"
)

func CreateLightUTXOs(block *common.Block) []*common.LightUTXO {
	var lightUTXOs []*common.LightUTXO
	for _, tx := range block.Txs {
		common.DebugLogger.Printf("Processing transaction block: %s - tx: %s\n", block.Hash, tx.Txid)
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				lightUTXOs = append(lightUTXOs, &common.LightUTXO{
					TxId:         tx.Txid,
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

	return lightUTXOs
}

func extractSpentTaprootPubKeysFromBlock(block *common.Block) []common.SpentUTXO {
	var spentUTXOs []common.SpentUTXO
	for _, tx := range block.Txs {
		spentUTXOs = append(spentUTXOs, extractSpentTaprootPubKeysFromTx(&tx, block)...)
	}

	return spentUTXOs
}

func extractSpentTaprootPubKeysFromTx(tx *common.Transaction, block *common.Block) []common.SpentUTXO {
	var spentUTXOs []common.SpentUTXO

	for _, vin := range tx.Vin {
		if vin.Coinbase != "" {
			continue
		}
		switch vin.Prevout.ScriptPubKey.Type {

		case "witness_v1_taproot":
			spentUTXOs = append(spentUTXOs, common.SpentUTXO{
				SpentIn:     tx.Txid,
				Txid:        vin.Txid,
				Vout:        vin.Vout,
				Value:       common.ConvertFloatBTCtoSats(vin.Prevout.Value),
				BlockHeight: block.Height,
				BlockHash:   block.Hash,
				Timestamp:   block.Timestamp,
			})
		default:
			continue
		}
	}

	return spentUTXOs
}
