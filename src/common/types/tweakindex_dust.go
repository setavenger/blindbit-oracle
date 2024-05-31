package types

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// TweakIndexDust stores a full index per blockHash and not separate entries like Tweak
// there is no transaction cut-through, so it will keep a full history.
// The tweaks will most likely not be sorted in any meaningful way and have no metadata attached.
// TweakIndexDust differs from TweakIndex as it has the highest value per tx stored as well
type TweakIndexDust struct {
	BlockHash   string        `json:"block_hash"`
	BlockHeight uint32        `json:"block_height"`
	Data        []TweakDusted `json:"data"`
}

// todo optimise this, there are overlapping features here.
//  Maybe some interface that can always produce HighestValue() uint64 and TweakData() [33]byte

type TweakData struct {
	Data  [33]byte
	Value uint64
}

func (t TweakData) HighestValue() uint64 {
	return t.Value
}

func (t TweakData) Tweak() [33]byte {
	return t.Data
}

func PairFactoryTweakIndexDust() Pair {
	var pair Pair = &TweakIndexDust{}
	return pair
}

const lengthDataTweakIndexDust = TweakDataLength + 8

func (v *TweakIndexDust) SerialiseKey() ([]byte, error) {
	return GetDBKeyTweakIndexDust(v.BlockHash)
}

// SerialiseData data representation is []byte(t_1 || t_2 || t_3 || ... || t_n)
// Where t is (tweak || highestValue) resulting in 41 bytes per t
// This means that the total length has to be (len(x) % 41 == 0)
func (v *TweakIndexDust) SerialiseData() ([]byte, error) {

	// todo can this be made more efficiently?
	totalLength := len(v.Data) * lengthDataTweakIndexDust
	flattened := make([]byte, 0, totalLength)

	for _, tweakDusted := range v.Data {
		var buffer bytes.Buffer
		data := tweakDusted.Tweak()
		buffer.Write(data[:])
		err := binary.Write(&buffer, binary.BigEndian, tweakDusted.HighestValue())
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		flattened = append(flattened, buffer.Bytes()...)
	}

	return flattened, nil
}

func (v *TweakIndexDust) DeSerialiseKey(key []byte) error {
	if len(key) != 32 {
		common.ErrorLogger.Printf("wrong key length: %+v", key)
		return errors.New("key is wrong length. should not happen")
	}

	v.BlockHash = hex.EncodeToString(key)

	return nil
}

func (v *TweakIndexDust) DeSerialiseData(data []byte) error {
	if len(data)%lengthDataTweakIndexDust != 0 {
		common.ErrorLogger.Printf("wrong data length: %+v", data)
		return errors.New("data is wrong length. should not happen")
	}

	numArrays := len(data) / lengthDataTweakIndexDust
	v.Data = make([]TweakDusted, numArrays)
	// Iterate and copy segments from the flat slice into the new array of arrays

	// idx counts the position in the byte array
	var idx int
	// i counts the elements
	for i := 0; i < numArrays; i++ {
		var tweakDusted TweakData
		copy(tweakDusted.Data[:], data[idx:idx+TweakDataLength])
		err := binary.Read(bytes.NewReader(data[idx+TweakDataLength:idx+TweakDataLength+8]), binary.BigEndian, &tweakDusted.Value)
		if err != nil {
			common.ErrorLogger.Println(err)
			return err
		}
		v.Data[i] = tweakDusted
		idx += lengthDataTweakIndexDust
	}
	return nil
}

// TweakIndexDustFromTweakArray builds a TweakIndexDust from a slice of Tweak
// comes without blockHash or height
func TweakIndexDustFromTweakArray(tweaksMap map[string]Tweak, block *Block) *TweakIndexDust {
	var index TweakIndexDust

	// can only panic hence no error output
	for _, tx := range block.Txs {
		if tweak, exists := tweaksMap[tx.Txid]; exists {
			dustedTweak := TweakData{Data: tweak.TweakData, Value: tweak.HighestValue}
			index.Data = append(index.Data, dustedTweak)
		}
	}

	return &index
}

// ToTweakArray creates a slice of Tweak from the TweakIndex
func (v *TweakIndexDust) ToTweakArray() []Tweak {
	var tweaks []Tweak
	// can only panic hence no error output
	for _, data := range v.Data {
		tweaks = append(tweaks, Tweak{
			BlockHash:    v.BlockHash,
			BlockHeight:  v.BlockHeight,
			Txid:         "", // cannot be determined as it's not stored in the index
			TweakData:    data.Tweak(),
			HighestValue: data.HighestValue(),
		})
	}
	return tweaks
}

func GetDBKeyTweakIndexDust(blockHash string) ([]byte, error) {
	var buf bytes.Buffer
	blockHashBytes, err := hex.DecodeString(blockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	buf.Write(blockHashBytes)

	return buf.Bytes(), nil
}
