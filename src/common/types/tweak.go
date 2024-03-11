package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/hex"
	"errors"
)

type Tweak struct {
	BlockHash   string   `json:"block_hash" bson:"block_hash"`
	BlockHeight uint32   `json:"block_height" bson:"block_height"`
	Txid        string   `json:"txid" bson:"txid"`
	Data        [33]byte `json:"data"`
}

func PairFactoryTweak() Pair {
	var filter Pair = &Tweak{}
	return filter
}

func (v *Tweak) SerialiseKey() ([]byte, error) {
	return GetDBKeyTweak(v.BlockHash, v.Txid)

}

func (v *Tweak) SerialiseData() ([]byte, error) {
	return v.Data[:], nil
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
	copy(v.Data[:], data)
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
