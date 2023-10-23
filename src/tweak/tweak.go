package tweak

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/p2p"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"log"
	"math/big"
	"sort"
	"strings"
)

func ComputeTweak(tx *common.Transaction) (*common.TweakData, error) {
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		// mainly relevant for test environments, unlikely on mainnet
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinbase {
			return nil, errors.New("coinbase transaction")
		}
		log.Printf("%+v\n", tx)
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

	tweakData := &common.TweakData{
		TxId:        tx.Txid,
		BlockHeight: tx.Status.BlockHeight,
		Data:        tweakBytes,
	}
	return tweakData, nil
}

func extractPubKeys(tx *common.Transaction) []string {
	var pubKeys []string

	for _, vin := range tx.Vin {
		switch strings.ToUpper(vin.Prevout.ScriptpubkeyType) {
		case "V1_P2TR":
			pubKeys = append(pubKeys, vin.Prevout.Scriptpubkey[4:])
		case "V0_P2WPKH":
			pubKeys = append(pubKeys, vin.Witness[1])
		case "P2SH-P2WPKH":
			pubKeys = append(pubKeys, vin.Witness[1])
		case "P2PKH":
			p2PKH, err := extractFromP2PKH(vin.Scriptsig)
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

	for _, op := range ops {
		// The byte sequence might represent a public key if it has the right length
		data, err := hex.DecodeString(op)
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		if len(data) == 33 || len(data) == 65 {
			return data, nil
		}
	}

	return nil, errors.New("no public key found in scriptSig")
}

func extractSpentTaprootPubKeys(tx *common.Transaction, ph *p2p.PeerHandler) []common.SpentUTXO {
	var vins []common.SpentUTXO

	for _, vin := range tx.Vin {
		switch strings.ToUpper(vin.Prevout.ScriptpubkeyType) {

		case "V1_P2TR":
			// todo what to do in cases of errors, for robustness they should be collected somewhere
			blockHashBytes, err := hex.DecodeString(tx.Status.BlockHash)
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			hash := chainhash.Hash{}
			err = hash.SetBytes(common.ReverseBytes(blockHashBytes))
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			vins = append(vins, common.SpentUTXO{
				SpentIn:     tx.Txid,
				Txid:        vin.Txid,
				Vout:        vin.Vout,
				Value:       vin.Prevout.Value,
				BlockHeight: tx.Status.BlockHeight,
				BlockHeader: tx.Status.BlockHash,
				Timestamp:   ph.GetTimestampByHeader(&hash),
			})
		default:
			continue
		}
	}

	return vins
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
