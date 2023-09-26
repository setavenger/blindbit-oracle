package common

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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
	//fmt.Println("Before reversing:", bytes)
	//for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
	//	bytes[i], bytes[j] = bytes[j], bytes[i]
	//}
	//fmt.Println("After reversing:", bytes)

	newHash, err := chainhash.NewHash(bytes[:32])
	if err != nil {
		panic(err)
	}
	return newHash

}

func IndexOfHashInHeaderList(element *chainhash.Hash, data []*Header) int32 {
	for i, v := range data {
		if *element == *v.Hash {
			return int32(i)
		}
	}
	return -1 // return -1 if the element is not found
}
