package types

import (
	"bytes"
	"encoding/hex"
	"errors"

	"SilentPaymentAppBackend/src/common"
)

const LenOutpointHashShort = 8

type SpentOutpointsIndex struct {
	BlockHash   string                       `json:"block_hash"`
	BlockHeight uint32                       `json:"block_height"`
	Data        [][LenOutpointHashShort]byte `json:"data"`
}

func PairFactorySpentOutpointsIndex() Pair {
	var filter Pair = &SpentOutpointsIndex{}
	return filter
}

func (v *SpentOutpointsIndex) SerialiseKey() ([]byte, error) {
	return GetDBKeyTweakIndex(v.BlockHash)
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
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.BlockHash = hex.EncodeToString(key)

	return nil
}

func (v *SpentOutpointsIndex) DeSerialiseData(data []byte) error {
	if len(data)%LenOutpointHashShort != 0 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}

	numArrays := len(data) / LenOutpointHashShort
	v.Data = make([][LenOutpointHashShort]byte, numArrays)
	// Iterate and copy segments from the flat slice into the new array of arrays
	for i := 0; i < numArrays; i++ {
		copy(v.Data[i][:], data[i*LenOutpointHashShort:(i+1)*LenOutpointHashShort])
	}
	return nil
}

func GetDBKeySpentSpentOutpointsIndex(blockHash string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}
