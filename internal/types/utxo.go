package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
)

// UTXO
// todo could be changed to unify spent UTXO and Light UTXO,
//
//	unused fields could just be omitted from serialisation and de-serialisation
type UTXO struct {
	Txid         [32]byte `json:"txid"`
	Vout         uint32   `json:"vout"`
	Value        uint64   `json:"value"`
	ScriptPubKey [34]byte `json:"scriptpubkey"`
	BlockHeight  uint32   `json:"block_height"` // not used
	BlockHash    [32]byte `json:"block_hash"`
	Timestamp    uint64   `json:"timestamp"` // not used
	Spent        bool     `json:"spent"`
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
	// todo: change to use fixed size slice make([]byte, 34) and put instead of a buffer
	var buf bytes.Buffer
	var err error

	buf.Write(v.ScriptPubKey[:])

	err = binary.Write(&buf, binary.BigEndian, v.Value)
	if err != nil {
		logging.L.Err(err).Msg("error serialising utxo")
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, v.Timestamp)
	if err != nil {
		logging.L.Err(err).Msg("error serialising utxo")
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, v.Spent)
	if err != nil {
		logging.L.Err(err).Msg("error serialising utxo")
		return nil, err
	}
	data := buf.Bytes()
	if len(data) != SerialisedDataLengthUtxo {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Int("length", len(data)).Msg("wrong data length")
		return nil, err
	}

	return data, nil
}

func (v *UTXO) DeSerialiseKey(key []byte) error {
	if len(key) != SerialisedKeyLengthUtxo {
		err := errors.New("key is wrong length. should not happen")
		logging.L.Err(err).Int("length", len(key)).Msg("wrong key length")
		return err
	}

	copy(v.BlockHash[:], key[:32])
	copy(v.Txid[:], key[32:64])
	err := binary.Read(bytes.NewReader(key[64:]), binary.BigEndian, &v.Vout)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising utxo")
		return err
	}
	return nil
}

func (v *UTXO) DeSerialiseData(data []byte) error {
	if len(data) != SerialisedDataLengthUtxo {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Int("length", len(data)).Msg("wrong data length")
		return err
	}
	v.ScriptPubKey = hex.EncodeToString(data[:34])
	err := binary.Read(bytes.NewReader(data[34:34+8]), binary.BigEndian, &v.Value)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising utxo")
		return err
	}
	err = binary.Read(bytes.NewReader(data[34+8:34+8+8]), binary.BigEndian, &v.Timestamp)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising utxo")
		return err
	}
	err = binary.Read(bytes.NewReader(data[34+8+8:]), binary.BigEndian, &v.Spent)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising utxo")
		return err
	}
	return nil
}

func GetDBKeyUTXO(blockHash, txid [32]byte, vout uint32) ([]byte, error) {
	key := make([]byte, 32+32+4)

	// Copy blockHash (32 bytes)
	copy(key[:32], blockHash[:])

	// Copy txid (32 bytes)
	copy(key[32:64], txid[:])

	// Write vout (4 bytes) in big-endian format
	binary.BigEndian.PutUint32(key[64:], vout)

	return key, nil
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
		// If the spent UTXO was the largest, return the max value among the remaining UTXOs.
		return &valueMax, nil
	} else {
		// If the spent UTXO was not the largest, no adjustment is needed.
		return nil, nil
	}
}
