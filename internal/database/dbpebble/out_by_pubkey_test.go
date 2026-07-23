package dbpebble

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestKeyOutByPubkeyLayout(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0xAB}, SizePubKey)
	txid := bytes.Repeat([]byte{0xCD}, SizeTxid)
	var vout uint32 = 7

	k := KeyOutByPubkey(pubkey, txid, vout)
	if len(k) != 1+SizePubKey+SizeTxid+SizeVout {
		t.Fatalf("bad key len %d", len(k))
	}
	if k[0] != KOutByPubkey {
		t.Fatalf("bad prefix %x", k[0])
	}
	if !bytes.Equal(k[1:1+SizePubKey], pubkey) {
		t.Fatal("pubkey mismatch")
	}
	if !bytes.Equal(k[1+SizePubKey:1+SizePubKey+SizeTxid], txid) {
		t.Fatal("txid mismatch")
	}
	if got := binary.BigEndian.Uint32(k[1+SizePubKey+SizeTxid:]); got != vout {
		t.Fatalf("vout mismatch %d", got)
	}
}

func TestValOutByPubkeyRoundTrip(t *testing.T) {
	v := ValOutByPubkey(123456789)
	if len(v) != SizeAmt {
		t.Fatalf("bad val len %d", len(v))
	}
	if got := binary.LittleEndian.Uint64(v); got != 123456789 {
		t.Fatalf("amount mismatch %d", got)
	}
}

func TestBoundsOutByPubkey(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0x10}, SizePubKey)
	lb, ub := BoundsOutByPubkey(pubkey)

	inKey := KeyOutByPubkey(pubkey, bytes.Repeat([]byte{0x00}, SizeTxid), 0)
	if bytes.Compare(lb, inKey) > 0 {
		t.Fatal("lb should be <= in-range key")
	}
	if bytes.Compare(inKey, ub) >= 0 {
		t.Fatal("in-range key should be < ub")
	}

	otherKey := KeyOutByPubkey(bytes.Repeat([]byte{0x11}, SizePubKey), bytes.Repeat([]byte{0x00}, SizeTxid), 0)
	if bytes.Compare(otherKey, ub) < 0 {
		t.Fatal("different pubkey should be outside bounds")
	}
}

func TestOutByPubkeyBackfillReformat(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0x22}, SizePubKey)
	txid := bytes.Repeat([]byte{0x33}, SizeTxid)
	var vout uint32 = 5
	var amount uint64 = 4242

	koutKey := KeyOut(txid, vout)
	koutVal, err := ValOut(amount, pubkey)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := outByPubkeyBackfillEntryFromOut(koutKey, koutVal)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entry.key[:], KeyOutByPubkey(pubkey, txid, vout)) {
		t.Fatal("reformatted key mismatch")
	}
	if !bytes.Equal(entry.value[:], ValOutByPubkey(amount)) {
		t.Fatal("reformatted value mismatch")
	}
}
