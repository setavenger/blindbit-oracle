package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// BlockHeader struct to hold relevant BlockHeader data
// todo change naming to be consistent?
type BlockHeader struct {
	Hash          string `bson:"hash"`
	PrevBlockHash string `bson:"previousblockhash"`
	Timestamp     uint64 `bson:"timestamp"`
	Height        uint32 `bson:"height"`
}

func PairFactoryBlockHeader() Pair {
	var filter Pair = &BlockHeader{}
	return filter
}

func (v *BlockHeader) SerialiseKey() ([]byte, error) {
	return GetDBKeyBlockHeader(v.Hash)

}

func (v *BlockHeader) SerialiseData() ([]byte, error) {
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.BigEndian, v.Timestamp)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, v.Height)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	blockHashBytes, err := hex.DecodeString(v.PrevBlockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}

func (v *BlockHeader) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.Hash = hex.EncodeToString(key)

	return nil
}

func (v *BlockHeader) DeSerialiseData(data []byte) error {
	err := binary.Read(bytes.NewReader(data[:8]), binary.BigEndian, &v.Timestamp)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	err = binary.Read(bytes.NewReader(data[8:12]), binary.BigEndian, &v.Height)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	v.Hash = hex.EncodeToString(data[12:])
	return nil
}

func GetDBKeyBlockHeader(blockHash string) ([]byte, error) {
	return hex.DecodeString(blockHash)
}

var GenesisBlock = BlockHeader{
	Hash:          "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
	PrevBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
	Timestamp:     1231006505,
	Height:        0,
}
