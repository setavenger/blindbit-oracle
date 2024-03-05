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
	"math/big"
	"sort"
)

func ComputeTweaksForBlock(block *common.Block) (common.TweakIndex, error) {
	var tweakIndex common.TweakIndex
	tweakIndex.BlockHeight = block.Height
	tweakIndex.BlockHash = block.Hash
	for _, tx := range block.Txs {
		if tx.Hash == "609e1214d499ca2a69f360cb6c829d25672b0d84937ae8f8052961d76514e05f" {
			fmt.Println("pause")
		}
		common.DebugLogger.Printf("Processing transaction block: %s - tx: %s\n", block.Hash, tx.Txid)
		for _, vout := range tx.Vout {
			if vout.ScriptPubKey.Type == "witness_v1_taproot" { // only compute tweak for txs with a taproot output
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
	common.DebugLogger.Println("computing tweak for:", tx.Txid)
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {

		// no need for all that if no pubkeys were extracted it means that the transaction does not have any eligible inputs

		// todo optimize light/spent utxo extraction to only store utxos that were validated through here
		/*
			// if the coinbase key exists and is not empty it's a coinbase transaction
			if tx.Vin[0].Coinbase != "" {
				common.DebugLogger.Printf("%s was coinbase\n", tx.Txid)
				return nil, nil
			}
			common.DebugLogger.Printf("%+v\n", tx)
			return nil, errors.New("no pub keys were extracted")
		*/
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
				//common.DebugLogger.Println("txid:", tx.Txid)
				//common.WarningLogger.Printf("Found a vin that is 22 bytes but does not match p2wpkh signature")
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
	common.DebugLogger.Printf("%s:%d - %+v", vin.Txid, vin.Vout, witnessStack)

	if len(witnessStack) >= 1 {
		// Remove annex if present
		if len(witnessStack) > 1 && witnessStack[len(witnessStack)-1] == "50" {
			witnessStack = witnessStack[:len(witnessStack)-1]
		}

		common.DebugLogger.Printf("%s:%d - %+v", vin.Txid, vin.Vout, witnessStack)

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
				// The internal key is the public key hash for P2TR
				return hex.EncodeToString(internalKey), nil
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
		if len(bytesPubKey) == 32 {
			bytesPubKey, err = common.Get33PubKeyFrom32(bytesPubKey)
			if err != nil {
				common.ErrorLogger.Println(err)
				// todo remove panics
				panic(err)
				return nil, err
			}
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

// Deprecated: not used
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
