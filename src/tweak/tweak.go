package tweak

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"sort"
	"strings"
)

func ComputeTweak(tx *common.Transaction) (*common.TweakData, error) {
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		fmt.Printf("%+v\n", tx)
		return nil, errors.New("no pub keys were extracted")
	}
	key, err := sumPublicKeys(pubKeys)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	hash, err := computeOutpointsHash(tx)
	if err != nil {
		return nil, err
	}
	curve := btcec.KoblitzCurve{}

	x, _ := curve.ScalarMult(key.X(), key.Y(), hash[:])

	// sometimes an uneven number hex string is returned, so we have to pad the zeros
	s := fmt.Sprintf("%x", x)
	if len(s)%2 != 0 {
		s = "0" + s
	}
	decodedString, err := hex.DecodeString(s)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	tweakBytes := [32]byte{}
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
			//	todo implement me
			panic("implement me")
		default:
			continue
		}
	}

	return pubKeys
}

func extractSpentTaprootPubKeys(tx *common.Transaction) []common.SpentUTXO {
	var vins []common.SpentUTXO

	for _, vin := range tx.Vin {
		switch strings.ToUpper(vin.Prevout.ScriptpubkeyType) {
		case "V1_P2TR":
			vins = append(vins, common.SpentUTXO{
				SpentIn:     tx.Txid,
				Txid:        vin.Txid,
				Vout:        vin.Vout,
				BlockHeight: tx.Status.BlockHeight,
				BlockHeader: tx.Status.BlockHash,
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
			common.Logger.Error(err.Error())
			panic(err)
			return nil, err
		}
		if len(bytesPubKey) == 32 {
			bytesPubKey = bytes.Join([][]byte{{0x02}, bytesPubKey}, []byte{})
		}
		publicKey, err := btcec.ParsePubKey(bytesPubKey)
		if err != nil {
			common.Logger.Error(err.Error())
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
			if len(sX)%2 != 0 {
				sX = "0" + sX
			}
			sY := fmt.Sprintf("%x", y)
			if len(sY)%2 != 0 {
				sY = "0" + sY
			}
			fmt.Println(fmt.Sprintf("04%x%x", sX, sY))
			decodeString, err = hex.DecodeString(fmt.Sprintf("04%s%s", sX, sY))
			if err != nil {
				common.Logger.Error(err.Error())
				panic(err)
				return nil, err
			}

			lastPubKey, err = btcec.ParsePubKey(decodeString)
			if err != nil {
				common.Logger.Error(err.Error())
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
			common.Logger.Error(err.Error())
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
