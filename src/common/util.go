package common

import (
	"crypto/sha256"
	"os/user"
	"strings"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ripemd160"
)

// ReverseBytes reverses the bytes inside the byte slice and returns the same slice. It does not return a copy.
func ReverseBytes(bytes []byte) []byte {
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	return bytes
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

func HashTagged(tag string, msg []byte) [32]byte {
	tagHash := sha256.Sum256([]byte(tag))
	data := append(tagHash[:], tagHash[:]...)
	data = append(data, msg...)
	return sha256.Sum256(data)
}

// Hash160 performs a RIPEMD160(SHA256(data)) hash on the given data
func Hash160(data []byte) []byte {
	sha256Hash := sha256.Sum256(data)
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:]) // Hash the SHA256 hash
	return ripemd160Hasher.Sum(nil)
}

func ResolvePath(path string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir

	return strings.Replace(path, "~", dir, 1)
}
