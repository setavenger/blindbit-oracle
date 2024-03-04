package core

import (
	"SilentPaymentAppBackend/src/common"
	"encoding/hex"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/gcs"
)

// BuildTaprootOnlyFilter creates the taproot only filter
func BuildTaprootOnlyFilter(block *common.Block) []byte {
	var taprootOutput [][]byte

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				scriptAsBytes, err := hex.DecodeString(vout.ScriptPubKey.Hex)
				if err != nil {
					common.DebugLogger.Printf("Failed to build taproot filter for block: %s (%d)\n", block.Hash, block.Height)
					common.ErrorLogger.Fatalln(err)
					return nil
				}
				taprootOutput = append(taprootOutput, scriptAsBytes)
			}
		}
	}

	c := chainhash.Hash{}
	err := c.SetBytes([]byte(block.Hash))
	if err != nil {
		common.DebugLogger.Println(block.Hash)
		common.ErrorLogger.Fatalln(err)
		return nil
	}
	key := builder.DeriveKey(&c)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutput)
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return nil
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return nil
	}
	return nBytes

}
