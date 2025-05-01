package core

import (
	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"
	"github.com/setavenger/blindbit-oracle/src/db/dblevel"
)

func ExtractNewUTXOs(block *types.Block, eligible map[string]struct{}) []types.UTXO {
	common.DebugLogger.Println("Getting new UTXOs")
	var utxos []types.UTXO
	for _, tx := range block.Txs {

		// only transactions with tweaks (pre-filtered by tweak computation) are going to be added
		_, ok := eligible[tx.Txid]
		if !ok {
			continue
		}
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
		// todo change switch to simple if statement
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
					common.ErrorLogger.Printf("prevout: %+v\n", vin.Prevout)
					common.ErrorLogger.Println("Headers not synced from first taproot like occurrence. Either build complete index or fully sync headers only.")
					panic(err)
				}
				blockHash = headerInv.Hash
			}

			spentUTXOs = append(spentUTXOs, types.UTXO{
				Txid:         vin.Txid,
				Vout:         vin.Vout,
				Value:        common.ConvertFloatBTCtoSats(vin.Prevout.Value),
				ScriptPubKey: vin.Prevout.ScriptPubKey.Hex,
				BlockHash:    blockHash,
				Spent:        true,
			})
		default:
			continue
		}
	}

	return spentUTXOs
}
