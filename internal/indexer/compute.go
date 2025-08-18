package indexer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"strings"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/go-bip352"
	golibsecp256k1 "github.com/setavenger/go-libsecp256k1"
)

func ComputeTweakPerTx(tx *Transaction) (*[33]byte, error) {
	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		// for example if coinbase transaction does not return any pubKeys (as it should)
		return nil, nil
	}

	fixSizePubKeys := utils.ConvertPubkeySliceToFixedLength33(pubKeys)

	summedKey, err := bip352.SumPublicKeys(fixSizePubKeys)
	if err != nil {
		if strings.Contains(err.Error(), "not on secp256k1 curve") {
			logging.L.Warn().Str("txid", tx.txid.String()).Err(err).Msg("error computing tweak per tx")
			return nil, nil
		}
		logging.L.Debug().Str("txid", tx.txid.String()).Msg("error computing tweak per tx")
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}
	// todo: bip352.ComputeInputHash(nil, nil) entire library needs interface first
	hash, err := ComputeInputHash(tx, summedKey)
	if err != nil {
		logging.L.Debug().Str("txid", tx.txid.String()).Msg("error computing tweak per tx")
		logging.L.Err(err).Msg("error computing tweak per tx")
		return nil, err
	}

	golibsecp256k1.PubKeyTweakMul(summedKey, &hash)

	tweakBytes := summedKey

	return tweakBytes, nil
}

// ComputeInputHash computes the input_hash for a transaction as per the specification.
func ComputeInputHash(tx *Transaction, sumPublicKeys *[33]byte) ([32]byte, error) {
	smallestOutpoint, err := findSmallestOutpoint(tx)
	if err != nil {
		logging.L.Err(err).Msg("error finding smallest outpoint")
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

func findSmallestOutpoint(tx *Transaction) ([]byte, error) {
	// think about returning *[36]byte

	// todo: remove this is basically not a valid
	if len(tx.ins) == 0 {
		return nil, errors.New("transaction has no inputs")
	}

	// Define a slice to hold the serialized outpoints
	outpoints := make([][36]byte, 0, len(tx.ins))

	for _, in := range tx.ins {
		// chainhash should be little endian in backend so we should be able to directly use it
		var outpoint [36]byte
		copy(outpoint[:32], in.txIn.PreviousOutPoint.Hash[:])

		// using a helper slice for vout/index as not sure on the exact copy and slicing behaviour of underlying AppendUint32
		var index [4]byte

		indexBytes := binary.LittleEndian.AppendUint32(index[:], in.txIn.PreviousOutPoint.Index)
		copy(outpoint[:], indexBytes)

		// Add the serialized outpoint to the slice
		outpoints = append(outpoints, outpoint)
	}

	// Sort the slice of outpoints to find the lexicographically smallest one
	sort.Slice(outpoints, func(i, j int) bool {
		return bytes.Compare(outpoints[i][:], outpoints[j][:]) < 0
	})

	// Return the smallest outpoint, if available
	if len(outpoints) > 0 {
		return outpoints[0][:], nil
	}

	return nil, errors.New("no valid outpoints found in transaction inputs, should not happen")
}
