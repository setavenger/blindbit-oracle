// Package types This is the inverse of the blockHeader in order to map blockHeight to blockHash/*
package types

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
)

// BlockHeaderInv struct to hold the inverse BlockHeader data.
// Required because we need different Serialisation for Pair interface
// todo change naming to be consistent?
type BlockHeaderInv struct {
	Hash   [32]byte
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
	// todo: this should be optimisable by using a fixed size byte arrays
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, v.Flag)
	if err != nil {
		logging.L.Err(err).Msg("error serialising block header inv")
		return nil, err
	}
	buf.Write(v.Hash[:])

	return buf.Bytes(), nil
}

func (v *BlockHeaderInv) DeSerialiseKey(key []byte) error {
	if len(key) != 4 {
		err := errors.New("key is wrong length. should not happen")
		logging.L.Err(err).Hex("key", key).Msg("wrong key length")
		return err
	}

	err := binary.Read(bytes.NewReader(key[:4]), binary.BigEndian, &v.Height)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising block header inv")
		return err
	}

	return nil
}

func (v *BlockHeaderInv) DeSerialiseData(data []byte) error {
	if len(data) != 1+32 {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Hex("data", data).Msg("wrong data length")
		return err
	}
	err := binary.Read(bytes.NewReader(data[:1]), binary.BigEndian, &v.Flag)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising block header inv")
		return err
	}
	copy(v.Hash[:], data[1:])
	return nil
}

func GetKeyBlockHeaderInv(height uint32) ([]byte, error) {
	var key [4]byte
	binary.BigEndian.PutUint32(key[:], height)
	return key[:], nil
}
