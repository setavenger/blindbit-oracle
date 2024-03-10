package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"fmt"
)

func CreateLightUTXOs(block *types.Block) []*types.LightUTXO {
	var lightUTXOs []*types.LightUTXO
	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				lightUTXOs = append(lightUTXOs, &types.LightUTXO{
					Txid:         tx.Txid,
					Vout:         vout.N,
					Value:        common.ConvertFloatBTCtoSats(vout.Value),
					ScriptPubKey: vout.ScriptPubKey.Hex,
					BlockHeight:  block.Height,
					BlockHash:    block.Hash,
					Timestamp:    block.Timestamp,
					TxidVout:     fmt.Sprintf("%s:%d", tx.Txid, vout.N),
				})
			}
		}
	}

	return lightUTXOs
}

func extractSpentTaprootPubKeysFromBlock(block *types.Block) []types.SpentUTXO {
	var spentUTXOs []types.SpentUTXO
	for _, tx := range block.Txs {
		spentUTXOs = append(spentUTXOs, extractSpentTaprootPubKeysFromTx(&tx, block)...)
	}

	return spentUTXOs
}

func extractSpentTaprootPubKeysFromTx(tx *types.Transaction, block *types.Block) []types.SpentUTXO {
	var spentUTXOs []types.SpentUTXO

	for _, vin := range tx.Vin {
		if vin.Coinbase != "" {
			continue
		}
		switch vin.Prevout.ScriptPubKey.Type {

		case "witness_v1_taproot":
			spentUTXOs = append(spentUTXOs, types.SpentUTXO{
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
