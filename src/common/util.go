package common

import (
	"crypto/sha256"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ripemd160"
	"math/big"
)

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

// Get33PubKeyFrom32 takes x coordinate without parity and returns compressed pub key
func Get33PubKeyFrom32(input []byte) ([]byte, error) {
	// Step 1: Convert the 32-byte input into an *big.Int
	xCoord := new(big.Int).SetBytes(input)

	// Step 2: Use the btcec package to get the curve and find y-coordinate
	curve := btcec.S256() // SECP256K1 curve
	yCoord, _ := curve.ScalarBaseMult(xCoord.Bytes())

	yCoord = yCoord.Mod(yCoord, curve.Params().P)

	// Step 3: Check the parity of the y-coordinate
	parityByte := byte(0x02) // Assume even y by default
	if yCoord.Bit(0) == 1 {  // Check if the last bit of y is 1 (odd)
		parityByte = 0x03 // Update for odd y
	}

	// Step 4: Prepend the parity byte to the original 32-byte array
	output := append([]byte{parityByte}, input...)

	// `output` is now your 33-byte array with the correct parity prepended
	return output, nil
}
