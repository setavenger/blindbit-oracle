package indexer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/setavenger/go-bip352"
	golibsecp256k1 "github.com/setavenger/go-libsecp256k1"
)

func ComputeTweakForTx(tx Transaction) (*types.Tweak, error) {
	pubKeys := ExtractPubKeys(tx.GetTxIns())

	summedKey, err := bip352.SumPublicKeys(pubKeys)
	if err != nil {
		logging.L.Err(err).Hex("txid", tx.GetTxIdSlice()).Msg("error computing tweak per tx")
		return nil, err
	}
	hash, err := ComputeInputHash(tx, summedKey)
	if err != nil {
		logging.L.Err(err).Hex("txid", tx.GetTxIdSlice()).Msg("error computing tweak per tx")
		return nil, err
	}

	golibsecp256k1.PubKeyTweakMul(summedKey, &hash)

	tweakBytes := summedKey // todo: can probably skip the reassignment

	highestValue, err := FindBiggestOutputFromTx(tx)
	if err != nil {
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}

	tweak := types.Tweak{
		Txid:         tx.GetTxId(),
		TweakData:    *tweakBytes,
		HighestValue: highestValue,
	}

	return &tweak, nil
}

func ExtractPubKeys(txIns []TxIn) [][33]byte {
	var pubKeys [][33]byte
	for _, txIn := range txIns {
		if !txIn.Valid() {
			continue
		}
		// logging.L.Debug().Hex("txid", txIn.GetTxIdSlice()).Uint32("vout", txIn.GetVout()).Any("txin", txIn).Msgf("extracting pubkeys: %+v", txIn)
		// logging.L.Debug().Hex("prevoutPkScript", txIn.GetPrevoutPkScript()).Msg("prevoutPkScript")
		// logging.L.Debug().Hex("scriptSig", txIn.GetScriptSig()).Msg("scriptSig")
		// logging.L.Debug().Any("witness", txIn.GetWitness()).Msg("witness")
		// logging.L.Debug().Any("valid", txIn.Valid()).Msg("valid")
		switch {
		case IsP2TR(txIn.GetPrevoutPkScript()):
			pubKey, err := extractPubKeyFromP2TR(txIn)
			if err != nil {
				logging.L.Err(err).Msg("error extracting public key from P2TR")
				continue
			}
			if len(pubKey) != 32 {
				continue
			}
			pubKey = bytes.Join([][]byte{{0x02}, pubKey}, nil) // prepend for always even parity
			pubKeys = append(pubKeys, utils.ConvertToFixedLength33(pubKey))
		case IsP2Wpkh(txIn.GetPrevoutPkScript()):
			witnessData := txIn.GetWitness()
			if len(witnessData[len(witnessData)-1]) == 33 {
				pubKey := witnessData[len(witnessData)-1]
				pubKeys = append(pubKeys, utils.ConvertToFixedLength33(pubKey))
			}
		case IsP2Sh(txIn.GetPrevoutPkScript()):
			scriptSig := txIn.GetScriptSig()
			witnessData := txIn.GetWitness()
			if len(scriptSig) == 23 {
				if scriptSig[0] == 0x16 && scriptSig[1] == 0x00 && scriptSig[2] == 0x14 {
					if len(witnessData[len(witnessData)-1]) == 33 {
						pubKeys = append(pubKeys, utils.ConvertToFixedLength33(witnessData[len(witnessData)-1]))
					}
				}
			}
		case IsP2Pkh(txIn.GetPrevoutPkScript()):
			pubKey, err := extractPubKeyFromP2PKH(txIn)
			if err != nil {
				logging.L.Err(err).Msg("error extracting public key from P2Pkh")
				continue
			}
			if len(pubKey) != 33 {
				continue
			}
			pubKeys = append(pubKeys, utils.ConvertToFixedLength33(pubKey))
		default:
			continue
		}
	}

	return pubKeys
}

var (
	P2TrPrefix   = []byte{0x51, 0x20}
	P2WpkhPrefix = []byte{0x00, 0x14}
)

func IsP2TR(script []byte) bool {
	// logging.L.Debug().Hex("script", script).Msg("IsP2TR")
	return bytes.Equal(script[:2], P2TrPrefix)
}

func IsP2Wpkh(script []byte) bool {
	// logging.L.Debug().Hex("script", script).Msg("IsP2Wpkh")
	return len(script) == 22 && bytes.Equal(script[:2], P2WpkhPrefix)
}

func IsP2Sh(script []byte) bool {
	return len(script) == 23 && script[0] == 0xa9 && script[1] == 0x14 && script[22] == 0x87
}

func IsP2Pkh(script []byte) bool {
	return len(script) == 25 &&
		script[0] == 0x76 && // OP_DUP
		script[1] == 0xa9 && // OP_HASH160
		script[2] == 0x14 && // OP_PUSHBYTES_20
		script[23] == 0x88 && // OP_EQUALVERIFY
		script[24] == 0xac // OP_CHECKSIG
}

// extractPublicKey tries to find a public key within the given scriptSig.
func extractPubKeyFromP2PKH(vin TxIn) ([]byte, error) {
	spkHash := vin.GetPrevoutPkScript()[3:23] // Skip op_codes and grab the hash

	scriptSig := vin.GetScriptSig()

	// todo: inefficient implementation copied from reference implementation
	//  should be improved upon
	for i := len(scriptSig); i >= 33; i-- {
		pubKeyBytes := scriptSig[i-33 : i]
		pubKeyHash := bip352.Hash160(pubKeyBytes)
		if bytes.Equal(pubKeyHash, spkHash) {
			return pubKeyBytes, nil
		}
	}

	return nil, nil
}

func extractPubKeyFromP2TR(vin TxIn) ([]byte, error) {
	witnessStack := vin.GetWitness()

	if len(witnessStack) >= 1 {
		prevout := vin.GetPrevoutPkScript()

		// Remove annex if present
		if len(witnessStack) > 1 && len(witnessStack[len(witnessStack)-1]) == 1 && witnessStack[len(witnessStack)-1][0] == 0x50 {
			witnessStack = witnessStack[:len(witnessStack)-1]
		}

		if len(witnessStack) > 1 {
			// Script-path spend
			controlBlock := witnessStack[len(witnessStack)-1]
			// Control block format: <control byte> <32-byte internal key> [<32-byte hash>...]
			if len(controlBlock) >= 33 {
				internalKey := controlBlock[1:33]

				if bytes.Equal(internalKey, bip352.NumsH) {
					// Skip if internal key is NUMS_H
					return nil, nil
				}

				return prevout[2:], nil
			}
		}

		return prevout[2:], nil
	}

	return nil, nil
}

// ComputeInputHash computes the input_hash for a transaction as per the specification.
func ComputeInputHash(tx Transaction, sumPublicKeys *[33]byte) ([32]byte, error) {
	smallestOutpoint, err := findSmallestOutpoint(tx)
	if err != nil {
		logging.L.Err(err).Hex("txid", tx.GetTxIdSlice()).Msg("error finding smallest outpoint")
		return [32]byte{}, err
	}

	// Concatenate outpointL and A
	var buffer bytes.Buffer
	buffer.Write(smallestOutpoint)
	// Serialize the x-coordinate of the sumPublicKeys
	buffer.Write(sumPublicKeys[:])

	inputHash := bip352.HashTagged("BIP0352/Inputs", buffer.Bytes())

	return inputHash, nil
}

func findSmallestOutpoint(tx Transaction) ([]byte, error) {
	vins := tx.GetTxIns()

	if len(vins) == 0 {
		return nil, errors.New("transaction has no inputs")
	}

	// Define a slice to hold the serialized outpoints
	outpoints := make([][]byte, 0, len(vins))

	for _, vin := range vins {
		// fmt.Printf("vin: %x\n", vin.GetTxId())
		// fmt.Printf("vin.GetVout(): %d\n", vin.GetVout())
		// fmt.Printf("vin.GetPrevoutPkScript(): %x\n", vin.GetPrevoutPkScript())
		// fmt.Printf("vin.GetScriptSig(): %x\n", vin.GetScriptSig())
		// fmt.Printf("vin.GetWitness(): %x\n", vin.GetWitness())
		// fmt.Printf("vin.Valid(): %t\n", vin.Valid())

		// Skip coinbase transactions as they do not have a regular prevout
		// make sure this check is still relevant
		// we might be already excluding coinbase outputs from the get go
		if !vin.Valid() {
			continue
		}
		// logging.L.Debug().Hex("txid", vin.GetTxIdSlice()).Msg("findSmallestOutpoint")
		// logging.L.Debug().Uint32("vout", vin.GetVout()).Msg("findSmallestOutpoint")

		// Decode the Txid (hex to bytes) and reverse it to match little-endian format
		txidBytes := vin.GetTxId()
		reversedTxid := utils.ReverseBytes(txidBytes[:]) // todo: check if txids are reversed already

		// Serialize the Vout as little-endian bytes
		voutBytes := new(bytes.Buffer)
		err := binary.Write(voutBytes, binary.LittleEndian, vin.GetVout())
		if err != nil {
			logging.L.Err(err).Msg("error serializing vout")
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

	return nil, errors.New("no valid outpoints found in transaction inputs, should not happen")
}

func FindBiggestOutputFromTx(tx Transaction) (uint64, error) {
	txOuts := tx.GetTxOuts()
	if len(txOuts) == 0 {
		return 0, errors.New("transaction has no outputs")
	}

	biggestOutput := txOuts[0]
	for _, txOut := range txOuts {
		if txOut.GetValue() > biggestOutput.GetValue() {
			biggestOutput = txOut
		}
	}

	return biggestOutput.GetValue(), nil
}
