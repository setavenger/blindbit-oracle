package types

import (
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
)

const LenOutpointHashShort = 8

type SpentOutpointsIndex struct {
	BlockHash   [32]byte                     `json:"block_hash"`
	BlockHeight uint32                       `json:"block_height"`
	Data        [][LenOutpointHashShort]byte `json:"data"`
}

func PairFactorySpentOutpointsIndex() Pair {
	var filter Pair = &SpentOutpointsIndex{}
	return filter
}

func (v *SpentOutpointsIndex) SerialiseKey() ([]byte, error) {
	return GetDBKeySpentOutpointsIndex(v.BlockHash)
}

func (v *SpentOutpointsIndex) SerialiseData() ([]byte, error) {

	// todo can this be made more efficiently?
	totalLength := len(v.Data) * LenOutpointHashShort
	flattened := make([]byte, 0, totalLength)

	for _, byteArray := range v.Data {
		flattened = append(flattened, byteArray[:]...)
	}

	return flattened, nil
}

func (v *SpentOutpointsIndex) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		err := errors.New("key is wrong length. should not happen")
		logging.L.Err(err).Hex("key", key).Msg("wrong key length")
		return err
	}

	copy(v.BlockHash[:], key)

	return nil
}

func (v *SpentOutpointsIndex) DeSerialiseData(data []byte) error {
	if len(data)%LenOutpointHashShort != 0 {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Hex("data", data).Msg("wrong data length")
		return err
	}

	numArrays := len(data) / LenOutpointHashShort
	v.Data = make([][LenOutpointHashShort]byte, numArrays)
	// Iterate and copy segments from the flat slice into the new array of arrays
	for i := 0; i < numArrays; i++ {
		copy(v.Data[i][:], data[i*LenOutpointHashShort:(i+1)*LenOutpointHashShort])
	}
	return nil
}

func GetDBKeySpentOutpointsIndex(blockHash [32]byte) ([]byte, error) {
	return blockHash[:], nil
}
