package core

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"math/big"
	"sort"
)

func ComputeTweaksForBlock(block *common.Block) ([]common.Tweak, error) {
	var tweaks []common.Tweak

	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			// only compute tweak for txs with a taproot output
			if vout.ScriptPubKey.Type == "witness_v1_taproot" {
				tweakPerTx, err := ComputeTweakPerTx(&tx)
				if err != nil {
					common.ErrorLogger.Println(err)
					return []common.Tweak{}, err
				}
				// we do this check for not eligible transactions like coinbase transactions
				// they are not supposed to throw an error
				// but also don't have a tweak that can be computed
				if tweakPerTx != nil {
					tweaks = append(tweaks, common.Tweak{
						BlockHash:   block.Hash,
						BlockHeight: block.Height,
						Txid:        tx.Txid,
						Data:        *tweakPerTx,
					})
				}
				break
			}
		}
	}
	return tweaks, nil
}

func ComputeTweakPerTx(tx *common.Transaction) (*[33]byte, error) {
	//common.DebugLogger.Println("computing tweak for:", tx.Txid)
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		return nil, nil
	}
	summedKey, err := sumPublicKeys(pubKeys)
	if err != nil {
		common.DebugLogger.Println("tx:", tx.Txid)
		common.ErrorLogger.Println(err)
		return nil, err
	}
	hash, err := ComputeInputHash(tx, summedKey)
	if err != nil {
		return nil, err
	}
	curve := btcec.KoblitzCurve{}

	x, y := curve.ScalarMult(summedKey.X(), summedKey.Y(), hash[:])

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
			pubKey, err := extractPubKeyHashFromP2TR(vin)
			if err != nil {
				common.DebugLogger.Println("txid:", tx.Txid)
				common.DebugLogger.Println("Could not extract public key")
				common.ErrorLogger.Println(err)
				continue
			}
			// todo what to do if none is matched
			if pubKey != "" {
				pubKeys = append(pubKeys, pubKey)
			}
		case "witness_v0_keyhash":
			// last element in the witness data is public key; skip uncompressed
			if len(vin.Txinwitness[len(vin.Txinwitness)-1]) == 66 {
				pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
			}

		case "scripthash":
			if len(vin.ScriptSig.Hex) == 46 {
				if vin.ScriptSig.Hex[:6] == "160014" {
					if len(vin.Txinwitness[len(vin.Txinwitness)-1]) == 66 {
						pubKeys = append(pubKeys, vin.Txinwitness[len(vin.Txinwitness)-1])
					}
				}
			}
		case "pubkeyhash":
			pubKey, err := extractFromP2PKH(vin)
			if err != nil {
				common.DebugLogger.Println("txid:", tx.Txid)
				common.DebugLogger.Println("Could not extract public key")
				common.ErrorLogger.Println(err)
				continue
			}

			// todo what to do if none is matched
			if pubKey != nil {
				pubKeys = append(pubKeys, hex.EncodeToString(pubKey))
			}

		default:
			continue
		}
	}

	return pubKeys
}

// extractPublicKey tries to find a public key within the given scriptSig.
func extractFromP2PKH(vin common.Vin) ([]byte, error) {
	// Assuming the scriptPubKey's hex starts with the op_codes and then the hash
	spkHashHex := vin.Prevout.ScriptPubKey.Hex[6:46] // Skip op_codes and grab the hash
	spkHash, err := hex.DecodeString(spkHashHex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	scriptSigBytes, err := hex.DecodeString(vin.ScriptSig.Hex)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	// todo inefficient implementation copied from reference implementation
	//  should be improved upon
	for i := len(scriptSigBytes); i >= 33; i-- {
		pubKeyBytes := scriptSigBytes[i-33 : i]
		pubKeyHash := common.Hash160(pubKeyBytes)
		if string(pubKeyHash) == string(spkHash) {
			return pubKeyBytes, err
		}
	}

	return nil, nil
}

func extractPubKeyHashFromP2TR(vin common.Vin) (string, error) {
	witnessStack := vin.Txinwitness

	if len(witnessStack) >= 1 {
		// Remove annex if present
		if len(witnessStack) > 1 && witnessStack[len(witnessStack)-1] == "50" {
			witnessStack = witnessStack[:len(witnessStack)-1]
		}

		if len(witnessStack) > 1 {
			// Script-path spend
			controlBlock, err := hex.DecodeString(witnessStack[len(witnessStack)-1])
			if err != nil {
				common.ErrorLogger.Println(err)
				return "", err
			}
			// Control block format: <control byte> <32-byte internal key> [<32-byte hash>...]
			if len(controlBlock) >= 33 {
				internalKey := controlBlock[1:33]

				if bytes.Equal(internalKey, common.NumsH) {
					// Skip if internal key is NUMS_H
					return "", nil
				}

				return vin.Prevout.ScriptPubKey.Hex[4:], nil
			}
		}

		return vin.Prevout.ScriptPubKey.Hex[4:], nil
	}

	return "", nil
}

func sumPublicKeys(pubKeys []string) (*btcec.PublicKey, error) {
	var lastPubKey *btcec.PublicKey
	curve := btcec.KoblitzCurve{}

	for idx, pubKey := range pubKeys {
		bytesPubKey, err := hex.DecodeString(pubKey)
		if err != nil {
			common.ErrorLogger.Println(err)
			// todo remove panics
			panic(err)
			return nil, err
		}

		// for extracted keys which are only 32 bytes (taproot) we assume even parity
		// as we don't need the y-coordinate for any computation we can simply prepend 0x02
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

// ComputeInputHash computes the input_hash for a transaction as per the specification.
func ComputeInputHash(tx *common.Transaction, sumPublicKeys *btcec.PublicKey) ([32]byte, error) {
	// Step 1: Aggregate public keys (A)

	// Step 2: Find the lexicographically smallest outpoint (outpointL)
	smallestOutpoint, err := findSmallestOutpoint(tx) // Implement this function based on your requirements
	if err != nil {
		return [32]byte{}, fmt.Errorf("error finding smallest outpoint: %w", err)
	}

	// Concatenate outpointL and A
	var buffer bytes.Buffer
	buffer.Write(smallestOutpoint) // Ensure this is serialized as per your transaction structure
	// Serialize the x-coordinate of the sumPublicKeys
	buffer.Write(sumPublicKeys.SerializeCompressed())

	// Compute input_hash using domain-separated hash
	inputHash := common.HashTagged("BIP0352/Inputs", buffer.Bytes())

	return inputHash, nil
}

func findSmallestOutpoint(tx *common.Transaction) ([]byte, error) {
	if len(tx.Vin) == 0 {
		return nil, errors.New("transaction has no inputs")
	}

	// Define a slice to hold the serialized outpoints
	outpoints := make([][]byte, 0, len(tx.Vin))

	for _, vin := range tx.Vin {
		// Skip coinbase transactions as they do not have a regular prevout
		if vin.Coinbase != "" {
			continue
		}

		// Decode the Txid (hex to bytes) and reverse it to match little-endian format
		txidBytes, err := hex.DecodeString(vin.Txid)
		if err != nil {
			return nil, err
		}
		reversedTxid := common.ReverseBytes(txidBytes)

		// Serialize the Vout as little-endian bytes
		voutBytes := new(bytes.Buffer)
		err = binary.Write(voutBytes, binary.LittleEndian, vin.Vout)
		if err != nil {
			common.ErrorLogger.Println(err)
			return nil, err
		}
		// Concatenate reversed Txid and Vout bytes
		outpoint := append(reversedTxid, voutBytes.Bytes()...)

		// Add the serialized outpoint to the slice
		outpoints = append(outpoints, outpoint)
	}

	// Sort the slice of outpoints to find the lexicographically smallest one
	sort.Slice(outpoints, func(i, j int) bool {
		return bytes.Compare(outpoints[i], outpoints[j]) < 0
	})

	// Return the smallest outpoint, if available
	if len(outpoints) > 0 {
		return outpoints[0], nil
	}

	return nil, errors.New("no valid outpoints found in transaction inputs")
}
