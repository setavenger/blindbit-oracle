package core

import (
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"

	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func ExtractNewUTXOs(block *types.Block, eligible map[string]struct{}) []*types.UTXO {
	logging.L.Trace().Msg("Getting new UTXOs")
	var utxos []*types.UTXO
	for _, tx := range block.Txs {
		// only transactions with tweaks (pre-filtered by tweak computation) are going to be added
		_, ok := eligible[tx.Txid]
		if !ok {
			continue
		}
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				value := utils.ConvertFloatBTCtoSats(vout.Value)
				utxos = append(utxos, &types.UTXO{
					Txid:         tx.Txid,
					Vout:         vout.N,
					Value:        value,
					ScriptPubKey: vout.ScriptPubKey.Hex,
					BlockHeight:  block.Height,
					BlockHash:    block.Hash,
					Timestamp:    block.Timestamp,
					Spent:        value == 0, // Mark as spent if value is 0
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
		if vin.Prevout.ScriptPubKey.Type == "witness_v1_taproot" {
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
					logging.L.Err(err).Msg("Failed to fetch by block height block header inv")
					logging.L.Debug().Any("tx", tx).Any("prevout", vin.Prevout).Msg("Failed to fetch by block height block header inv")
					// panic because if this fails it means we have incomplete data which requires a sync
					logging.L.Panic().Err(err).Msg("Headers not synced from first taproot like occurrence. Either build complete index or fully sync headers only.")
					return nil
				}
				blockHash = headerInv.Hash
			}

			spentUTXOs = append(spentUTXOs, types.UTXO{
				Txid:         vin.Txid,
				Vout:         vin.Vout,
				Value:        utils.ConvertFloatBTCtoSats(vin.Prevout.Value),
				ScriptPubKey: vin.Prevout.ScriptPubKey.Hex,
				BlockHash:    blockHash,
				Spent:        true,
			})
		} else {
			continue
		}
	}

	return spentUTXOs
}
