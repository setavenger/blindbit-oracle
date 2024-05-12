package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"encoding/hex"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/gcs"
)

// BuildTaprootOnlyFilter creates the taproot only filter
func BuildTaprootOnlyFilter(block *types.Block) (types.Filter, error) {
	var taprootOutput [][]byte

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				scriptAsBytes, err := hex.DecodeString(vout.ScriptPubKey.Hex)
				if err != nil {
					common.DebugLogger.Printf("Failed to build taproot filter for block: %s (%d)\n", block.Hash, block.Height)
					common.ErrorLogger.Fatalln(err)
					return types.Filter{}, err
				}
				// only append the x-only pubKey. reduces complexity
				taprootOutput = append(taprootOutput, scriptAsBytes[2:])
			}
		}
	}

	blockHashBytes, err := hex.DecodeString(block.Hash)
	if err != nil {
		common.DebugLogger.Println("blockHash", block.Hash)
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err
	}
	c := chainhash.Hash{}

	err = c.SetBytes(common.ReverseBytes(blockHashBytes))
	if err != nil {
		common.DebugLogger.Println("blockHash", block.Hash)
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err

	}
	key := builder.DeriveKey(&c)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutput)
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err
	}

	return types.Filter{
		FilterType:  4,
		BlockHeight: block.Height,
		Data:        nBytes,
		BlockHash:   block.Hash,
	}, nil
}
