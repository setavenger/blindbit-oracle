package core

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

// BuildTaprootOnlyFilter creates the taproot only filter
func BuildNewUTXOsFilter(block *types.Block) (types.Filter, error) {
	var taprootOutput [][]byte

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				scriptAsBytes, err := hex.DecodeString(vout.ScriptPubKey.Hex)
				if err != nil {
					logging.L.Fatal().
						Err(err).
						Str("blockhash", block.Hash).
						Uint32("height", block.Height).
						Msg("Failed to build taproot filter for block")
					return types.Filter{}, err
				}
				// only append the x-only pubKey. reduces complexity
				taprootOutput = append(taprootOutput, scriptAsBytes[2:])
			}
		}
	}

	blockHashBytes, err := hex.DecodeString(block.Hash)
	if err != nil {
		logging.L.Fatal().Err(err).Str("blockhash", block.Hash).Msg("Failed to decode block hash")
		return types.Filter{}, err
	}
	c := chainhash.Hash{}

	err = c.SetBytes(utils.ReverseBytes(blockHashBytes))
	if err != nil {
		logging.L.Fatal().Err(err).Str("blockhash", block.Hash).Msg("Failed to set block hash")
		return types.Filter{}, err
	}
	key := builder.DeriveKey(&c)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutput)
	if err != nil {
		logging.L.Fatal().Err(err).Str("blockhash", block.Hash).Msg("Failed to build GCS filter")
		return types.Filter{}, err
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		logging.L.Fatal().Err(err).Str("blockhash", block.Hash).Msg("Failed to get NBytes")
		return types.Filter{}, err
	}

	blockhashBytes, err := hex.DecodeString(block.Hash)
	if err != nil {
		logging.L.Fatal().Err(err).Str("blockhash", block.Hash).Msg("Failed to decode block hash")
		return types.Filter{}, err
	}

	return types.Filter{
		FilterType:  4,
		BlockHeight: block.Height,
		Data:        nBytes,
		BlockHash:   [32]byte(blockhashBytes),
	}, nil
}

// BuildSpentUTXOsFilter creates a filter based on the spent
func BuildSpentUTXOsFilter(spentOutpointsIndex types.SpentOutpointsIndex) (types.Filter, error) {
	c := chainhash.Hash{}

	err := c.SetBytes(utils.ReverseBytes(spentOutpointsIndex.BlockHash[:]))
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", spentOutpointsIndex.BlockHash[:]).Msg("Failed to set block hash")
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
		logging.L.Fatal().Err(err).Hex("blockhash", spentOutpointsIndex.BlockHash[:]).Msg("Failed to build GCS filter")
		return types.Filter{}, err
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", spentOutpointsIndex.BlockHash[:]).Msg("Failed to get NBytes")
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
	out := make([]byte, 32+4)

	copy(out[:32], utils.ReverseBytesCopy(utxo.Txid[:]))
	binary.LittleEndian.PutUint32(out[32:], utxo.Vout)

	return out, nil
}
