package indexer

import (
	"bytes"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/go-bip352"
)

func extractPubKeys(tx *Transaction) [][]byte {
	// todo: we should probably prepend to even pubkey and return slice of 33byte arrays
	var pubKeys [][]byte

	for _, in := range tx.ins {
		switch {
		case bip352.IsP2TR(in.prevOut.PkScript):
			// todo needs some extra parsing see reference implementation and bitcoin core wallet
			pubKey, err := extractPubKeyFromP2TR(in)
			if err != nil {
				logging.L.Debug().Str("txid", tx.txid.String()).Msg("Could not extract public key")
				logging.L.Panic().Err(err).Msg("Could not extract public key")
				return nil
			}

			// todo: what to do if none is matched
			if len(pubKey) == 32 {
				// todo: we should probably prepend to even pubkey and return slice of 33byte arrays
				pubKeys = append(pubKeys, pubKey)
			}

		case bip352.IsP2WPKH(in.prevOut.PkScript):
			// last element in the witness data is public key; skip uncompressed
			if len(in.txIn.Witness[len(in.txIn.Witness)-1]) == 33 {
				pubKeys = append(pubKeys, in.txIn.Witness[len(in.txIn.Witness)-1])
			}

		case bip352.IsP2SH(in.prevOut.PkScript):
			if len(in.txIn.SignatureScript) == 23 {
				if bytes.Equal(in.txIn.SignatureScript[:3], []byte{0x16, 0x00, 0x14}) {
					// if vin.ScriptSig.Hex[:6] == "160014" {
					lastElem := in.txIn.Witness[len(in.txIn.Witness)-1]
					if len(lastElem) == 33 {
						pubKeys = append(pubKeys, lastElem)
					}
				}
			}

		case bip352.IsP2PKH(in.prevOut.PkScript):
			pubKey, err := extractFromP2PKH(in)
			if err != nil {
				logging.L.Debug().Str("txid", tx.txid.String()).Msg("Could not extract public key")
				logging.L.Err(err).Msg("Could not extract public key")
				continue
			}
			// todo what to do if none is matched
			if pubKey != nil {
				pubKeys = append(pubKeys, pubKey)
			}
		default:
			continue
		}
	}

	return pubKeys
}

func extractPubKeyFromP2TR(vin *Vin) ([]byte, error) {
	witnessStack := vin.txIn.Witness

	if len(witnessStack) >= 1 {
		// Remove annex if present
		if len(witnessStack) > 1 && bytes.Equal(witnessStack[len(witnessStack)-1], []byte{0x50}) {
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

				return vin.prevOut.PkScript[2:], nil
			}
		}

		return vin.prevOut.PkScript[2:], nil
	}

	return nil, nil
}

func extractFromP2PKH(in *Vin) ([]byte, error) {
	spkHash := in.prevOut.PkScript[3:23] // Skip op_codes and grab the hash

	// todo: inefficient implementation copied from reference implementation
	//  should be improved upon
	for i := len(in.txIn.SignatureScript); i >= 33; i-- {
		pubKeyBytes := in.txIn.SignatureScript[i-33 : i]
		pubKeyHash := bip352.Hash160(pubKeyBytes)
		if bytes.Equal(pubKeyHash, spkHash) {
			return pubKeyBytes, nil
		}
	}

	return nil, nil
}
