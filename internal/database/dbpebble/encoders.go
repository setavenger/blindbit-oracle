package dbpebble

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/setavenger/blindbit-lib/logging"
)

// ---------------- Keys ----------------

func KeyBlockTx(blockHash []byte, pos uint32) []byte {
	k := make([]byte, 1+SizeHash+SizePos)
	k[0] = KBlockTx
	copy(k[1:1+SizeHash], blockHash)
	be32(pos, k[1+SizeHash:1+SizeHash+SizePos])
	return k
}

func BoundsBlockTx(blockHash []byte) (lb, ub []byte) {
	lb = make([]byte, 1+SizeHash+SizePos)
	lb[0] = KBlockTx
	copy(lb[1:1+SizeHash], blockHash)
	// pos = 0x00000000
	ub = make([]byte, 1+SizeHash+SizePos)
	ub[0] = KBlockTx
	copy(ub[1:1+SizeHash], blockHash)
	// pos = 0xFFFFFFFF  (exclusive upper bound; practically safe)
	for i := 1 + SizeHash; i < 1+SizeHash+SizePos; i++ {
		ub[i] = 0xFF
	}
	return
}

func KeyTx(txid []byte) []byte {
	k := make([]byte, 1+SizeTxid)
	k[0] = KTx
	copy(k[1:], txid)
	return k
}

func KeyTxOccur(txid, blockHash []byte) []byte {
	k := make([]byte, 1+SizeTxid+SizeHash)
	k[0] = KTxOccur
	copy(k[1:1+SizeTxid], txid)
	copy(k[1+SizeTxid:], blockHash)
	return k
}

func KeyOut(txid []byte, vout uint32) []byte {
	k := make([]byte, 1+SizeTxid+SizeVout)
	k[0] = KOut
	copy(k[1:1+SizeTxid], txid)
	be32(vout, k[1+SizeTxid:])
	return k
}

func BoundsOut(txid []byte) (lb, ub []byte) {
	lb = make([]byte, 1+SizeTxid+SizeVout)
	lb[0] = KOut
	copy(lb[1:1+SizeTxid], txid)
	ub = make([]byte, 1+SizeTxid+SizeVout)
	ub[0] = KOut
	copy(ub[1:1+SizeTxid], txid)
	for i := 1 + SizeTxid; i < 1+SizeTxid+SizeVout; i++ {
		ub[i] = 0xFF
	}
	return
}

func KeySpend(prevTxid []byte, prevVout uint32, blockHash []byte) []byte {
	k := make([]byte, 1+SizeTxid+SizeVout+SizeHash)
	k[0] = KSpend
	copy(k[1:1+SizeTxid], prevTxid)
	be32(prevVout, k[1+SizeTxid:1+SizeTxid+SizeVout])
	copy(k[1+SizeTxid+SizeVout:], blockHash)
	return k
}

func BoundsSpend(prevTxid []byte, prevVout uint32) (lb, ub []byte) {
	lb = make([]byte, 1+SizeTxid+SizeVout+SizeHash)
	lb[0] = KSpend
	copy(lb[1:1+SizeTxid], prevTxid)
	be32(prevVout, lb[1+SizeTxid:1+SizeTxid+SizeVout])
	// blockHash = 00..00
	ub = make([]byte, 1+SizeTxid+SizeVout+SizeHash)
	copy(ub, lb)
	for i := 1 + SizeTxid + SizeVout; i < len(ub); i++ {
		ub[i] = 0xFF
	}
	return
}

func KeyCIHeight(height uint32) []byte {
	k := make([]byte, 1+SizeHeight)
	k[0] = KCIHeight
	be32(height, k[1:])
	return k
}

func BoundsCIHeight() (lb, ub []byte) {
	lb = make([]byte, 1+SizeHeight)
	lb[0] = KCIHeight
	be32(0, lb[1:])

	ub = make([]byte, 1+SizeHeight)
	ub[0] = KCIHeight
	be32(math.MaxUint32, ub[1:])
	return
}

func KeyCIBlock(blockHash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KCIBlock
	copy(k[1:], blockHash)
	return k
}

func KeySpentOutputsShort(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KSpentOutputsShort
	copy(k[1:], blockhash)
	return k
}

// ---------------- Txid Outpoints Mapping ----------------

func KeyTxidOutpoints(blockhash, txid []byte) []byte {
	k := make([]byte, 1+SizeHash+SizeTxid)
	k[0] = KTxidOutpoints
	copy(k[1:1+SizeHash], blockhash)
	copy(k[1+SizeHash:], txid)
	return k
}

func BoundsTxidOutpoints(blockhash []byte) (lb, ub []byte) {
	lb = make([]byte, 1+SizeHash+SizeTxid)
	lb[0] = KTxidOutpoints
	copy(lb[1:1+SizeHash], blockhash)
	// txid = 0x00000000...

	ub = make([]byte, 1+SizeHash+SizeTxid)
	ub[0] = KTxidOutpoints
	copy(ub[1:1+SizeHash], blockhash)
	// txid = 0xFFFFFFFF... (exclusive upper bound)
	for i := 1 + SizeHash; i < 1+SizeHash+SizeTxid; i++ {
		ub[i] = 0xFF
	}
	return
}

// ---------------- Compute Index ----------------

func KeyComputeIndex(height uint32, txid []byte) []byte {
	k := make([]byte, 1+SizeHeight+SizeTxid)
	k[0] = KComputeIndex
	be32(height, k[1:1+SizeHeight])
	copy(k[1+SizeHeight:], txid)
	return k
}

func BoundsComputeIndex(startHeight, endHeight uint32) (lb, ub []byte) {
	lb = make([]byte, 1+SizeHeight+SizeTxid)
	lb[0] = KComputeIndex
	be32(startHeight, lb[1:1+SizeHeight])
	ub = make([]byte, 1+SizeHeight+SizeTxid)
	ub[0] = KComputeIndex
	be32(endHeight, ub[1:1+SizeHeight])
	return
}

func BoundsComputeIndexOneHeight(height uint32) (lb, ub []byte) {
	lb = make([]byte, 1+SizeHeight+SizeTxid)
	lb[0] = KComputeIndex
	be32(height, lb[1:1+SizeHeight])
	ub = make([]byte, 1+SizeHeight+SizeTxid)
	ub[0] = KComputeIndex
	be32(height, ub[1:1+SizeHeight])
	for i := 1 + SizeHeight; i < 1+SizeHeight+SizeTxid; i++ {
		ub[i] = 0xFF
	}
	return lb, ub
}

// ---------------- Values ----------------

func ValTxTweak(tweak []byte) ([]byte, error) {
	if len(tweak) != SizeTweak {
		return nil, errors.New("tweak must be 33 bytes")
	}
	v := make([]byte, SizeTweak)
	copy(v, tweak)
	return v, nil
}

func ValOut(amount uint64, pubkey []byte) ([]byte, error) {
	if len(pubkey) != SizePubKey {
		return nil, errors.New("pubkey must be 32 bytes (x-only)")
	}
	v := make([]byte, SizeAmt+SizePubKey)
	le64(amount, v[:SizeAmt])
	copy(v[SizeAmt:], pubkey)
	return v, nil
}

func ParseOutValue(v []byte) (amount uint64, pubkey []byte, err error) {
	// todo: should maybe just panic wrong values should no exist.
	// If wrong values are in the db it's a development bug which should be caught fast
	if len(v) != SizeAmt+SizePubKey {
		return 0, nil, errors.New("bad out value length")
	}
	amount = binary.LittleEndian.Uint64(v[:SizeAmt])
	pk := make([]byte, SizePubKey)
	copy(pk, v[SizeAmt:])
	return amount, pk, nil
}

func ValSpend(spendPub []byte) ([]byte, error) {
	if spendPub == nil {
		return nil, nil // keys-only
	}
	if len(spendPub) != SizePubKey {
		return nil, errors.New("spend pubkey must be 32 bytes")
	}
	v := make([]byte, SizePubKey)
	copy(v, spendPub)
	return v, nil
}

func ValTxidOutpoints(outpoints [][36]byte) ([]byte, error) {
	if len(outpoints) == 0 {
		return []byte{}, nil // empty array
	}

	// Each outpoint is 36 bytes (32-byte txid + 4-byte vout)
	value := make([]byte, 36*len(outpoints))
	for i, outpoint := range outpoints {
		copy(value[i*36:(i+1)*36], outpoint[:])
	}
	return value, nil
}

func ParseTxidOutpointsValue(v []byte) ([][36]byte, error) {
	if len(v) == 0 {
		return [][36]byte{}, nil // empty array
	}

	if len(v)%36 != 0 {
		err := errors.New("bad txid outpoints value length - must be multiple of 36")
		logging.L.Err(err).Hex("value", v).Msg("bad values in txid outpoints")
		return nil, err
	}

	numOutpoints := len(v) / 36
	outpoints := make([][36]byte, numOutpoints)
	for i := range numOutpoints {
		copy(outpoints[i][:], v[i*36:(i+1)*36])
	}
	return outpoints, nil
}
