package types

import (
	"bytes"
	"encoding/hex"
	"errors"

	"github.com/setavenger/blindbit-lib/logging"
)

// TweakIndex stores a full index per blockHash and not separate entries like Tweak
// there is no transaction cut-through, so it will keep a full history.
// The tweaks will most likely not be sorted in any meaningful way and have no metadata attached.
type TweakIndex struct {
	BlockHash   string                  `json:"block_hash"`
	BlockHeight uint32                  `json:"block_height"`
	Data        [][TweakDataLength]byte `json:"data"`
}

func PairFactoryTweakIndex() Pair {
	var filter Pair = &TweakIndex{}
	return filter
}

func (v *TweakIndex) SerialiseKey() ([]byte, error) {
	return GetDBKeyTweakIndex(v.BlockHash)

}

func (v *TweakIndex) SerialiseData() ([]byte, error) {

	// todo can this be made more efficiently?
	totalLength := len(v.Data) * TweakDataLength
	flattened := make([]byte, 0, totalLength)

	for _, byteArray := range v.Data {
		flattened = append(flattened, byteArray[:]...)
	}

	return flattened, nil
}

func (v *TweakIndex) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		err := errors.New("key is wrong length. should not happen")
		logging.L.Err(err).Hex("key", key).Msg("wrong key length")
		return err
	}

	v.BlockHash = hex.EncodeToString(key)

	return nil
}

func (v *TweakIndex) DeSerialiseData(data []byte) error {
	if len(data)%TweakDataLength != 0 {
		err := errors.New("data is wrong length. should not happen")
		logging.L.Err(err).Hex("data", data).Msg("wrong data length")
		return err
	}

	numArrays := len(data) / TweakDataLength
	v.Data = make([][TweakDataLength]byte, numArrays)
	// Iterate and copy segments from the flat slice into the new array of arrays
	for i := 0; i < numArrays; i++ {
		copy(v.Data[i][:], data[i*TweakDataLength:(i+1)*TweakDataLength])
	}
	return nil
}

// TweakIndexFromTweakArray builds a TweakIndex from a slice of Tweak
// comes without blockHash or height
func TweakIndexFromTweakArray(tweaksMap map[string]Tweak, block *Block) *TweakIndex {
	// todo benchmark the sorting, should not create too much overhead,
	//  seems more like a nice to have for comparisons across implementations
	var index TweakIndex
	// can only panic hence no error output
	for _, tx := range block.Txs {
		if tweak, exists := tweaksMap[tx.Txid]; exists {
			index.Data = append(index.Data, tweak.TweakData)
		}
	}
	return &index
}

// ToTweakArray creates a slice of Tweak from the TweakIndex
func (v *TweakIndex) ToTweakArray() (tweaks []Tweak) {
	// can only panic hence no error output
	for _, data := range v.Data {
		tweaks = append(tweaks, Tweak{
			BlockHash:   v.BlockHash,
			BlockHeight: v.BlockHeight,
			Txid:        "",
			TweakData:   data,
		})
	}
	return
}

func GetDBKeyTweakIndex(blockHash string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		logging.L.Err(err).Hex("blockHash", []byte(blockHash)).Msg("error decoding block hash")
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}
