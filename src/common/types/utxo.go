package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/setavenger/blindbit-oracle/src/common"
)

// UTXO
// todo could be changed to unify spent UTXO and Light UTXO,
//
//	unused fields could just be omitted from serialisation and de-serialisation
type UTXO struct {
	Txid         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Value        uint64 `json:"value"`
	ScriptPubKey string `json:"scriptpubkey"`
	BlockHeight  uint32 `json:"block_height"` // not used
	BlockHash    string `json:"block_hash"`
	Timestamp    uint64 `json:"timestamp"` // not used
	Spent        bool   `json:"spent"`
}

const SerialisedKeyLengthUtxo = 32 + 32 + 4
const SerialisedDataLengthUtxo = 34 + 8 + 8 + 1

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
	err = binary.Write(&buf, binary.BigEndian, v.Timestamp)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, v.Spent)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	data := buf.Bytes()
	if len(data) != SerialisedDataLengthUtxo {
		common.ErrorLogger.Printf("wrong data length: %d %+v", len(data), data)
		return nil, err
	}

	return data, nil
}

func (v *UTXO) DeSerialiseKey(key []byte) error {
	if len(key) != SerialisedKeyLengthUtxo {
		common.ErrorLogger.Printf("wrong key length: %d %+v", len(key), key)
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
	if len(data) != SerialisedDataLengthUtxo {
		common.ErrorLogger.Printf("wrong data length: %d %+v", len(data), data)
		return errors.New("data is wrong length. should not happen")
	}
	v.ScriptPubKey = hex.EncodeToString(data[:34])
	err := binary.Read(bytes.NewReader(data[34:34+8]), binary.BigEndian, &v.Value)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	err = binary.Read(bytes.NewReader(data[34+8:34+8+8]), binary.BigEndian, &v.Timestamp)
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	err = binary.Read(bytes.NewReader(data[34+8+8:]), binary.BigEndian, &v.Spent)
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

// FindBiggestRemainingUTXO returns nil if the spent utxo was not the largest and
// hence no downward adjustment has to be made for the tweak.
// Returns the largest value of utxos if utxoSpent had the largest value.
func FindBiggestRemainingUTXO(utxoSpent UTXO, utxos []UTXO) (*uint64, error) {
	var valueMax uint64 = 0
	spentIsMax := false

	for _, utxo := range utxos {
		if utxo.Spent {
			continue
		}
		if utxo.Value < utxoSpent.Value {
			spentIsMax = true // Found a UTXO larger than the spent one.
		} else {
			spentIsMax = false
			valueMax = 0 // reset value max to zero as it's not the biggest anymore
			break        // break because it turns out it's not the biggest so our job here is done
		}

		if utxo.Value > valueMax {
			valueMax = utxo.Value // Update max value found among remaining UTXOs.
		}
	}

	if spentIsMax {
		if valueMax == 0 {
			common.ErrorLogger.Printf("%+v", utxoSpent)
			common.ErrorLogger.Printf("%+v", utxos)
			return nil, errors.New("valueMax was 0. this should not happen")
		}
		// If the spent UTXO was the largest, return the max value among the remaining UTXOs.
		return &valueMax, nil
	} else {
		// If the spent UTXO was not the largest, no adjustment is needed.
		return nil, nil
	}
}
