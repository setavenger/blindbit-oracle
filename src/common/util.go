package common

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/shopspring/decimal"
)

func ReverseBytes(bytes []byte) []byte {
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	return bytes
}

func GetChainHash(hash chainhash.Hash) *chainhash.Hash {
	bytes, err := hex.DecodeString(hash.String())
	if err != nil {
		panic(err)
	}
	bytes = bytes[:32]
	//log.Println("Before reversing:", bytes)
	//for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
	//	bytes[i], bytes[j] = bytes[j], bytes[i]
	//}
	//log.Println("After reversing:", bytes)

	newHash, err := chainhash.NewHash(bytes[:32])
	if err != nil {
		panic(err)
	}
	return newHash

}

func IndexOfHashInHeaderList(element *chainhash.Hash, data []*Header) int32 {
	for i, v := range data {
		if *element == *v.BlockHash {
			return int32(i)
		}
	}
	return -1 // return -1 if the element is not found
}

func ConvertFloatBTCtoSats(value float64) uint64 {
	valueBTC := decimal.NewFromFloat(value)
	satsConstant := decimal.NewFromInt(100_000_000)
	// Multiply the BTC value by the number of Satoshis per Bitcoin
	resultInDecimal := valueBTC.Mul(satsConstant)
	// Get the integer part of the result
	resultInInt := resultInDecimal.IntPart()
	// Convert the integer result to uint64 and return
	if resultInInt < 0 {
		DebugLogger.Println("value:", value, "result:", resultInInt)
		ErrorLogger.Fatalln("value was converted to negative value")
	}

	return uint64(resultInInt)
}
