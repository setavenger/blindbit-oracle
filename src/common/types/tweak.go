package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

type Tweak struct {
	BlockHash string `json:"block_hash"`
	// BlockHeight todo not really used at the moment, could be added on a per request basis in the API handler
	BlockHeight uint32   `json:"block_height"`
	Txid        string   `json:"txid"`
	Data        [33]byte `json:"data"`
	// HighestValue indicates the value of the UTXO with the most value for a specific tweak
	HighestValue uint64
}

func PairFactoryTweak() Pair {
	var filter Pair = &Tweak{}
	return filter
}

func (v *Tweak) SerialiseKey() ([]byte, error) {
	return GetDBKeyTweak(v.BlockHash, v.Txid)
}

func (v *Tweak) SerialiseData() ([]byte, error) {
	var buf bytes.Buffer

	buf.Write(v.Data[:])

	err := binary.Write(&buf, binary.BigEndian, v.HighestValue)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *Tweak) DeSerialiseKey(key []byte) error {
	if len(key) != 64 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.BlockHash = hex.EncodeToString(key[:32])
	v.Txid = hex.EncodeToString(key[32:])

	return nil
}

func (v *Tweak) DeSerialiseData(data []byte) error {
	if len(data) != 33 && len(data) != 33+8 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}
	copy(v.Data[:], data[:33])

	// we only try to read HighestValue if the data is there
	if len(data) == 33+8 {
		err := binary.Read(bytes.NewReader(data[33:]), binary.BigEndian, &v.HighestValue)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
	}

	return nil
}

func GetDBKeyTweak(blockHash, txid string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	txidBytes, err := hex.DecodeString(txid)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)
	buf.Write(txidBytes)

	return buf.Bytes(), nil
}
