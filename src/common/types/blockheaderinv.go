// Package types This is the inverse of the blockHeader in order to map blockHeight to blockHash/*
package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// todo we could amend this type to also include a flag on whether this block has already been processed.
//  We could have a complete list of all heights mapped to the hashes
//  and then a signaling bit to see whether its been processed or not

// BlockHeaderInv struct to hold the inverse BlockHeader data
// Needed because we need different Serialisation for Pair interface
// todo change naming to be consistent?
type BlockHeaderInv struct {
	Hash   string `bson:"hash"`
	Height uint32 `bson:"height"`
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
	if len(data) != 32 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}
	v.Hash = hex.EncodeToString(data)
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
