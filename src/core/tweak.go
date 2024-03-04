package core

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"math/big"
	"sort"
	"strings"
)

func ComputeTweaksForBlock(block *common.Block) (common.TweakIndex, error) {
	var tweakIndex common.TweakIndex
	tweakIndex.BlockHeight = block.Height
	tweakIndex.BlockHash = block.Hash
	for _, tx := range block.Txs {
		common.DebugLogger.Printf("Processing transaction block: %s - tx: %s\n", block.Hash, tx.Hash)
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				tweakPerTx, err := ComputeTweakPerTx(&tx)
				if err != nil {
					common.ErrorLogger.Println(err)
					return common.TweakIndex{}, nil
				}
				// we do this check for the coinbase transactions which are not supposed to throw an error
				// but also don't have a tweak that can be computed
				if tweakPerTx != nil {
					tweakIndex.Data = append(tweakIndex.Data, *tweakPerTx)
				}
				break
			}
		}
	}
	return tweakIndex, nil
}

func ComputeTweakPerTx(tx *common.Transaction) (*[33]byte, error) {
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		// if the coinbase key exists and is not empty it's a coinbase transaction
		if tx.Vin[0].Coinbase != "" {
			common.DebugLogger.Printf("%s was coinbase\n", tx.Hash)
			return nil, nil
		}
		common.DebugLogger.Printf("%+v\n", tx)
		return nil, errors.New("no pub keys were extracted")
	}
	key, err := sumPublicKeys(pubKeys)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	hash, err := computeOutpointsHash(tx)
	if err != nil {
		return nil, err
	}
	curve := btcec.KoblitzCurve{}

	x, y := curve.ScalarMult(key.X(), key.Y(), hash[:])

	// sometimes an uneven number hex string is returned, so we have to pad the zeros
	s := fmt.Sprintf("%x", x)
	s = fmt.Sprintf("%064s", s)
	mod := y.Mod(y, big.NewInt(2))
	if mod.Cmp(big.NewInt(0)) == 0 {
		s = "02" + s
	} else {
		s = "03" + s
	}

	decodedString, err := hex.DecodeString(s)

	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}
	tweakBytes := [33]byte{}
	copy(tweakBytes[:], decodedString)

	return &tweakBytes, nil
}

func extractPubKeys(tx *common.Transaction) []string {
	var pubKeys []string

	for _, vin := range tx.Vin {
		switch vin.Prevout.ScriptPubKey.Type {
		case "witness_v1_taproot":
			// todo needs some extra parsing see reference implementation and bitcoin core wallet
			pubKeys = append(pubKeys, vin.Prevout.ScriptPubKey.Hex[4:])
		case "witness_v0_keyhash":
			// last element in the witness data is public key
			pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
		case "scripthash":
			if len(vin.ScriptSig.ASM) == 44 {
				if vin.ScriptSig.ASM[:4] == "0014" {
					pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
				}
				common.WarningLogger.Printf("Found a vin that is 22 bytes but does not match p2wpkh signature")
			}
		case "pubkeyhash":
			p2PKH, err := extractFromP2PKH(vin.Prevout.ScriptPubKey.Hex)
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			pubKeys = append(pubKeys, hex.EncodeToString(p2PKH))
		default:
			continue
		}
	}

	return pubKeys
}

// extractPublicKey tries to find a public key within the given scriptSig.
func extractFromP2PKH(scriptSig string) ([]byte, error) {
	decodeString, err := hex.DecodeString(scriptSig)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// Disassemble the script into its string representation
	disassembled, err := txscript.DisasmString(decodeString)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// Split the disassembled string by space to get individual operations
	ops := strings.Split(disassembled, " ")

	// todo not enough, still possible to have errors
	for _, op := range ops {
		// The byte sequence might represent a public key if it has the right length
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		if len(op) == 66 {
			return hex.DecodeString(op)
		}
	}

	return nil, errors.New("no public key found in scriptSig")
}

func sumPublicKeys(pubKeys []string) (*btcec.PublicKey, error) {
	var lastPubKey *btcec.PublicKey
	curve := btcec.KoblitzCurve{}

	for idx, pubKey := range pubKeys {
		bytesPubKey, err := hex.DecodeString(pubKey)
		if err != nil {
			common.ErrorLogger.Println(err)
			panic(err)
			return nil, err
		}
		if len(bytesPubKey) == 32 {
			bytesPubKey = bytes.Join([][]byte{{0x02}, bytesPubKey}, []byte{})
		}
		publicKey, err := btcec.ParsePubKey(bytesPubKey)
		if err != nil {
			common.ErrorLogger.Println(err)
			panic(err)
			return nil, err
		}

		if idx == 0 {
			lastPubKey = publicKey
		} else {
			var decodeString []byte
			x, y := curve.Add(lastPubKey.X(), lastPubKey.Y(), publicKey.X(), publicKey.Y())

			// in case big int omits leading zero
			sX := fmt.Sprintf("%x", x)
			sY := fmt.Sprintf("%x", y)
			sX = fmt.Sprintf("%064s", sX)
			sY = fmt.Sprintf("%064s", sY)
			common.DebugLogger.Println(fmt.Sprintf("%s", sX))
			common.DebugLogger.Println(fmt.Sprintf("%s", sY))
			common.DebugLogger.Println(fmt.Sprintf("04%s%s", sX, sY))
			decodeString, err = hex.DecodeString(fmt.Sprintf("04%s%s", sX, sY))
			if err != nil {
				common.ErrorLogger.Println(err)
				panic(err)
				return nil, err
			}

			lastPubKey, err = btcec.ParsePubKey(decodeString)
			if err != nil {
				common.ErrorLogger.Println(err)
				panic(err)
				return nil, err
			}
		}
	}
	return lastPubKey, nil
}

func computeOutpointsHash(tx *common.Transaction) ([32]byte, error) {
	var completeBuffer [][]byte
	for _, vin := range tx.Vin {
		nBuf := new(bytes.Buffer)
		err := binary.Write(nBuf, binary.LittleEndian, vin.Vout)
		if err != nil {
			common.ErrorLogger.Println(err)
			return [32]byte{}, err
		}
		txIdBytes, err := hex.DecodeString(vin.Txid)
		if err != nil {
			return [32]byte{}, err
		}
		out := txIdBytes
		out = common.ReverseBytes(out)
		out = append(out, nBuf.Bytes()...)
		completeBuffer = append(completeBuffer, out)
	}

	sort.Slice(completeBuffer, func(i, j int) bool {
		return bytes.Compare(completeBuffer[i], completeBuffer[j]) < 0
	})

	// Join the byte slices together
	var combined []byte
	for _, d := range completeBuffer {
		combined = append(combined, d...)
	}

	return sha256.Sum256(combined), nil
}
