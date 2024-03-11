package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

type Filter struct {
	FilterType  uint8  `json:"filter_type" bson:"filter_type"`
	BlockHeight uint32 `json:"block_height" bson:"block_height"`
	Data        []byte `json:"data" bson:"data"`
	BlockHash   string `json:"block_hash" bson:"block_hash"`
}

func PairFactoryFilter() Pair {
	var filter Pair = &Filter{}
	return filter
}

func (v *Filter) SerialiseKey() ([]byte, error) {
	return GetDBKeyFilter(v.BlockHash)
}

func (v *Filter) SerialiseData() ([]byte, error) {
	var buf bytes.Buffer

	// start with filter type as that's fixed length
	err := binary.Write(&buf, binary.BigEndian, v.FilterType)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	buf.Write(v.Data)
	return buf.Bytes(), nil
}

func (v *Filter) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}
	// The block hash is fixed length, decode the block hash part
	v.BlockHash = hex.EncodeToString(key)

	return nil
}

func (v *Filter) DeSerialiseData(data []byte) error {
	v.FilterType = data[0]
	v.Data = data[1:]
	return nil
}

func GetDBKeyFilter(blockHash string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)
	return buf.Bytes(), nil
}
