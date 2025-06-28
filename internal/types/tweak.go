package types

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
)

const TweakDataLength = 33

type Tweak struct {
	BlockHash [32]byte `json:"block_hash"`
	// BlockHeight todo not really used at the moment, could be added on a per request basis in the API handler
	BlockHeight uint32                `json:"block_height"`
	Txid        [32]byte              `json:"txid"`
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
		logging.L.Err(err).Msg("error serialising tweak")
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *Tweak) DeSerialiseKey(key []byte) error {
	if len(key) != 64 {
		err := errors.New("key is wrong length. should not happen")
		logging.L.Err(err).Hex("key", key).Msg("wrong key length")
		return err
	}

	copy(v.BlockHash[:], key[:32])
	copy(v.Txid[:], key[32:])

	return nil
}

func (v *Tweak) DeSerialiseData(data []byte) error {
	// todo why did we check both dusted and non dusted
	if len(data) != TweakDataLength+8 {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Hex("data", data).Msg("wrong data length")
		return err
	}
	copy(v.TweakData[:], data[:TweakDataLength])

	// we only try to read HighestValue if the data is there
	// revoke: if the data is not there it seems like an implementation error. prior, where dust was an option it made sense
	err := binary.Read(bytes.NewReader(data[TweakDataLength:]), binary.BigEndian, &v.HighestValue)
	if err != nil {
		logging.L.Err(err).Msg("error deserialising tweak")
		return err
	}

	return nil
}

func GetDBKeyTweak(blockHash [32]byte, txid [32]byte) ([]byte, error) {
	key := make([]byte, 64)
	copy(key[:32], blockHash[:])
	copy(key[32:], txid[:])
	return key, nil
}
