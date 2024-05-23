package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"crypto/sha256"
	"encoding/hex"
)

func BuildSpentUTXOIndex(utxos []types.UTXO, block *types.Block) (types.SpentOutpointsIndex, error) {

	blockHashBytes, err := hex.DecodeString(block.Hash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return types.SpentOutpointsIndex{}, err
	}

	// reverse byte order to make little endian
	blockHashBytes = common.ReverseBytes(blockHashBytes)

	spentOutpointsIndex := types.SpentOutpointsIndex{
		BlockHash:   block.Hash,
		BlockHeight: block.Height,
	}

	for _, utxo := range utxos {
		var outpoint []byte
		outpoint, err = SerialiseToOutpoint(utxo)
		if err != nil {
			common.ErrorLogger.Println(err)
			return types.SpentOutpointsIndex{}, err
		}

		hashedOutpoint := sha256.Sum256(append(outpoint, blockHashBytes...))
		spentOutpointsIndex.Data = append(spentOutpointsIndex.Data, [types.LenOutpointHashShort]byte(hashedOutpoint[:]))
	}

	return spentOutpointsIndex, err
}
