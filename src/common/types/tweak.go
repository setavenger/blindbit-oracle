package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

const TweakDataLength = 33

type Tweak struct {
	BlockHash string `json:"block_hash"`
	// BlockHeight todo not really used at the moment, could be added on a per request basis in the API handler
	BlockHeight uint32                `json:"block_height"`
	Txid        string                `json:"txid"`
	TweakData   [TweakDataLength]byte `json:"tweak_data"`
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

	buf.Write(v.TweakData[:])

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
	// todo why did we check both dusted and non dusted
	if len(data) != TweakDataLength+8 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}
	copy(v.TweakData[:], data[:TweakDataLength])

	// we only try to read HighestValue if the data is there
	// revoke: if the data is not there it seems like an implementation error. prior, where dust was an option it made sense
	err := binary.Read(bytes.NewReader(data[TweakDataLength:]), binary.BigEndian, &v.HighestValue)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
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
