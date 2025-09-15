package dbpebble

import (
	"encoding/binary"
	"errors"
	"math"
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

// ---------------- Statics ----------------

func KeyTweaksStatic(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KTweaksStatic
	copy(k[1:], blockhash)
	return k
}

func KeyKUTXOsStatic(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KUTXOsStatic
	copy(k[1:], blockhash)
	return k
}

// ---------------- Filters ----------------

func KeyTaprootPubkeyFilter(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KTaprootPubkeyFilter
	copy(k[1:], blockhash)
	return k
}

func KeyTaprootUnspentFilter(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KTaprootUnspentFilter
	copy(k[1:], blockhash)
	return k
}

func KeyTaprootSpentFilter(blockhash []byte) []byte {
	k := make([]byte, 1+SizeHash)
	k[0] = KTaprootSpentFilter
	copy(k[1:], blockhash)
	return k
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
