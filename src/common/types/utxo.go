package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// UTXO could
// todo could be changed to unify spent UTXO and Light UTXO,
//  unused fields could just be omitted from serialisation and de-serialisation
type UTXO struct {
	Txid         string `json:"txid" bson:"txid"`
	Vout         uint32 `json:"vout" bson:"vout"`
	Value        uint64 `json:"value" bson:"value"`
	ScriptPubKey string `json:"scriptpubkey" bson:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height" bson:"block_height"`
	BlockHash    string `json:"block_hash" bson:"block_hash"`
	Timestamp    uint64 `json:"timestamp" bson:"timestamp"`
	Spent        bool   `json:"-"`
}

// SpentUTXO
// todo remove
// Deprecated: won't be stored and can hence be modified or replaced by a different struct type
type SpentUTXO struct {
	SpentIn     string `json:"spent_in" bson:"spentin"`
	Txid        string `json:"txid" bson:"txid"`
	Vout        uint32 `json:"vout" bson:"vout"`
	Value       uint64 `json:"value" bson:"value"`
	BlockHeight uint32 `json:"block_height" bson:"block_height"`
	BlockHash   string `json:"block_hash" bson:"block_hash"`
	Timestamp   uint64 `json:"timestamp" bson:"timestamp"`
}

func PairFactoryUTXO() Pair {
	var filter Pair = &UTXO{}
	return filter
}

func (v *UTXO) SerialiseKey() ([]byte, error) {
	return GetDBKeyUTXO(v.BlockHash, v.Txid, v.Vout)
}

func (v *UTXO) SerialiseData() ([]byte, error) {
	var buf bytes.Buffer
	scriptPubKeyBytes, err := hex.DecodeString(v.ScriptPubKey)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	buf.Write(scriptPubKeyBytes)

	err = binary.Write(&buf, binary.BigEndian, v.Value)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *UTXO) DeSerialiseKey(key []byte) error {
	if len(key) != 32+32+4 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.BlockHash = hex.EncodeToString(key[:32])
	v.Txid = hex.EncodeToString(key[32:64])
	err := binary.Read(bytes.NewReader(key[64:]), binary.BigEndian, &v.Vout)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return nil
}

func (v *UTXO) DeSerialiseData(data []byte) error {
	if len(data) != 34+8 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}
	v.ScriptPubKey = hex.EncodeToString(data[:34])
	err := binary.Read(bytes.NewReader(data[34:]), binary.BigEndian, &v.Value)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	return nil
}

func GetDBKeyUTXO(blockHash, txid string, vout uint32) ([]byte, error) {
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

	err = binary.Write(&buf, binary.BigEndian, vout)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	return buf.Bytes(), nil
}
