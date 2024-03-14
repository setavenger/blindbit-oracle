package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/hex"
	"errors"
)

// TweakIndex stores a full index per blockHash and not separate entries like Tweak
// there is no transaction cut-through, so it will keep a full history.
// The tweaks will most likely not be sorted in any meaningful way and have no metadata attached.
type TweakIndex struct {
	BlockHash   string     `json:"block_hash"`
	BlockHeight uint32     `json:"block_height"`
	Data        [][33]byte `json:"data"`
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
	totalLength := len(v.Data) * 33
	flattened := make([]byte, 0, totalLength)

	for _, byteArray := range v.Data {
		flattened = append(flattened, byteArray[:]...)
	}

	return flattened, nil
}

func (v *TweakIndex) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.BlockHash = hex.EncodeToString(key)

	return nil
}

func (v *TweakIndex) DeSerialiseData(data []byte) error {
	if len(data)%33 != 0 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}

	numArrays := len(data) / 33
	v.Data = make([][33]byte, numArrays)
	// Iterate and copy segments from the flat slice into the new array of arrays
	for i := 0; i < numArrays; i++ {
		copy(v.Data[i][:], data[i*33:(i+1)*33])
	}
	return nil
}

// TweakIndexFromTweakArray builds a TweakIndex from a slice of Tweak
// comes without blockHash or height
func TweakIndexFromTweakArray(tweaks []Tweak) *TweakIndex {
	var index TweakIndex
	// can only panic hence no error output
	for _, tweak := range tweaks {
		index.Data = append(index.Data, tweak.Data)
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
			Data:        data,
		})
	}
	return
}

func GetDBKeyTweakIndex(blockHash string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}
