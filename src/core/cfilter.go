package core

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"

	"github.com/setavenger/blindbit-oracle/src/common"
	"github.com/setavenger/blindbit-oracle/src/common/types"

	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/gcs"
)

// BuildTaprootOnlyFilter creates the taproot only filter
func BuildNewUTXOsFilter(block *types.Block) (types.Filter, error) {
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

// BuildSpentUTXOsFilter creates a filter based on the spent
func BuildSpentUTXOsFilter(spentOutpointsIndex types.SpentOutpointsIndex) (types.Filter, error) {

	blockHashBytes, err := hex.DecodeString(spentOutpointsIndex.BlockHash)
	if err != nil {
		common.DebugLogger.Println("blockHash", spentOutpointsIndex.BlockHash)
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err
	}
	c := chainhash.Hash{}

	err = c.SetBytes(common.ReverseBytes(blockHashBytes))
	if err != nil {
		common.DebugLogger.Println("blockHash", spentOutpointsIndex.BlockHash)
		common.ErrorLogger.Fatalln(err)
		return types.Filter{}, err

	}
	key := builder.DeriveKey(&c)

	// convert to slices
	data := make([][]byte, len(spentOutpointsIndex.Data))
	for i, outpointHash := range spentOutpointsIndex.Data {
		var newHash [8]byte
		copy(newHash[:], outpointHash[:])
		data[i] = newHash[:]
	}

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, data)
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
		BlockHeight: spentOutpointsIndex.BlockHeight,
		Data:        nBytes,
		BlockHash:   spentOutpointsIndex.BlockHash,
	}, nil
}

func SerialiseToOutpoint(utxo types.UTXO) ([]byte, error) {
	var buf bytes.Buffer

	txidBytes, err := hex.DecodeString(utxo.Txid)
	if err != nil {
		common.DebugLogger.Println(utxo.Txid)
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// err is always nil
	buf.Write(common.ReverseBytes(txidBytes))

	binary.Write(&buf, binary.LittleEndian, utxo.Vout)
	return buf.Bytes(), err
}
