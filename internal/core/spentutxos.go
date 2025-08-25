package core

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func BuildSpentUTXOIndex(utxos []types.UTXO, block *types.Block) (types.SpentOutpointsIndex, error) {

	blockHashBytes, err := hex.DecodeString(block.Hash)
	if err != nil {
		logging.L.Err(err).Msg("error decoding block hash")
		return types.SpentOutpointsIndex{}, err
	}

	// reverse byte order to make little endian
	blockHashBytes = utils.ReverseBytes(blockHashBytes)

	spentOutpointsIndex := types.SpentOutpointsIndex{
		BlockHash:   utils.ConvertToFixedLength32(blockHashBytes),
		BlockHeight: block.Height,
	}

	for _, utxo := range utxos {
		var outpoint []byte
		outpoint, err = SerialiseToOutpoint(utxo)
		if err != nil {
			logging.L.Err(err).Msg("error serialising to outpoint")
			return types.SpentOutpointsIndex{}, err
		}

		hashedOutpoint := sha256.Sum256(append(outpoint, blockHashBytes...))
		spentOutpointsIndex.Data = append(spentOutpointsIndex.Data, [types.LenOutpointHashShort]byte(hashedOutpoint[:]))
	}

	return spentOutpointsIndex, err
}
