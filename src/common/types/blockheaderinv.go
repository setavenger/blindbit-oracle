// Package types This is the inverse of the blockHeader in order to map blockHeight to blockHash/*
package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
)

// BlockHeaderInv struct to hold the inverse BlockHeader data.
// Required because we need different Serialisation for Pair interface
// todo change naming to be consistent?
type BlockHeaderInv struct {
	Hash   string
	Height uint32
	Flag   bool // indicates whether this Block has been processed
}

func PairFactoryBlockHeaderInv() Pair {
	var pair Pair = &BlockHeaderInv{}
	return pair
}

func (v *BlockHeaderInv) SerialiseKey() ([]byte, error) {
	return GetKeyBlockHeaderInv(v.Height)

}

func (v *BlockHeaderInv) SerialiseData() ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, v.Flag)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	blockHashBytes, err := hex.DecodeString(v.Hash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}

func (v *BlockHeaderInv) DeSerialiseKey(key []byte) error {
	if len(key) != 4 {
		common.ErrorLogger.Printf("wrong key length: %+v\n", key)
		return errors.New("key is wrong length. should not happen")
	}

	err := binary.Read(bytes.NewReader(key[:4]), binary.BigEndian, &v.Height)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}

	return nil
}

func (v *BlockHeaderInv) DeSerialiseData(data []byte) error {
	if len(data) != 1+32 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}
	err := binary.Read(bytes.NewReader(data[:1]), binary.BigEndian, &v.Flag)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	v.Hash = hex.EncodeToString(data[1:])
	return nil
}

func GetKeyBlockHeaderInv(height uint32) ([]byte, error) {
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.BigEndian, height)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return buf.Bytes(), nil
}
