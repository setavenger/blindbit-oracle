package dbpebble

import (
	"bytes"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

// setupOut writes a KOut row and its matching KOutByPubkey accelerator row,
// mirroring the live-write path.
func setupOut(t *testing.T, s *Store, txid, pubkey []byte, vout uint32, amount uint64) {
	t.Helper()
	val, err := ValOut(amount, pubkey)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Set(KeyOut(txid, vout), val, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Set(KeyOutByPubkey(pubkey, txid, vout), ValOutByPubkey(amount), nil); err != nil {
		t.Fatal(err)
	}
}

func setupComputeIndex(t *testing.T, s *Store, height uint32, txid, tweak []byte, shorts [][8]byte) {
	t.Helper()
	ci := ComputeIndex{TxId: txid, Height: height, Tweak: tweak, OutputsShort: shorts}
	if err := s.DB.Set(ci.SerialiseKey(), ci.SerialiseData(), nil); err != nil {
		t.Fatal(err)
	}
}

func short8(pubkey []byte) [8]byte {
	var s [8]byte
	copy(s[:], pubkey[:8])
	return s
}

func TestFetchComputeIndexDedupTaproot(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	defer s.Close()

	const height uint32 = 100
	tweak := bytes.Repeat([]byte{0xAA}, SizeTweak)

	txidA := bytes.Repeat([]byte{0x0A}, SizeTxid)
	txidB := bytes.Repeat([]byte{0x0B}, SizeTxid)
	txidC := bytes.Repeat([]byte{0x0C}, SizeTxid)

	p1 := bytes.Repeat([]byte{0x01}, SizePubKey)   // unique
	p2 := bytes.Repeat([]byte{0x02}, SizePubKey)   // unique
	pDup := bytes.Repeat([]byte{0x03}, SizePubKey) // funded twice (tx B + tx C)

	// tx A: single unique output
	setupOut(t, s, txidA, p1, 0, 1000)
	setupComputeIndex(t, s, height, txidA, tweak, [][8]byte{short8(p1)})

	// tx B: unique output (vout 0) + duplicate output (vout 1)
	setupOut(t, s, txidB, p2, 0, 2000)
	setupOut(t, s, txidB, pDup, 1, 3000)
	setupComputeIndex(t, s, height, txidB, tweak, [][8]byte{short8(p2), short8(pDup)})

	// tx C: only the duplicate pubkey -> should be dropped entirely
	setupOut(t, s, txidC, pDup, 0, 4000)
	setupComputeIndex(t, s, height, txidC, tweak, [][8]byte{short8(pDup)})

	items, stats, err := s.FetchComputeIndexDedupTaproot(height)
	if err != nil {
		t.Fatal(err)
	}

	if stats.TxsTotal != 3 {
		t.Fatalf("TxsTotal = %d, want 3", stats.TxsTotal)
	}
	if stats.TxsDropped != 1 {
		t.Fatalf("TxsDropped = %d, want 1", stats.TxsDropped)
	}
	if stats.TotalOutputs != 4 {
		t.Fatalf("TotalOutputs = %d, want 4", stats.TotalOutputs)
	}
	if stats.OmittedOutputs != 2 {
		t.Fatalf("OmittedOutputs = %d, want 2", stats.OmittedOutputs)
	}
	if stats.SentOutputs != 2 {
		t.Fatalf("SentOutputs = %d, want 2", stats.SentOutputs)
	}

	// tx C dropped -> 2 items remain
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	// Collect served short pubkeys and confirm pDup never served.
	for _, it := range items {
		if len(it.OutputsShort)%8 != 0 {
			t.Fatalf("OutputsShort length %d not multiple of 8", len(it.OutputsShort))
		}
		for off := 0; off < len(it.OutputsShort); off += 8 {
			if bytes.Equal(it.OutputsShort[off:off+8], pDup[:8]) {
				t.Fatal("duplicate-funded pubkey was served")
			}
		}
	}
}

func TestFetchComputeIndexDedupTaprootAllUnique(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	defer s.Close()

	const height uint32 = 7
	tweak := bytes.Repeat([]byte{0xBB}, SizeTweak)
	txid := bytes.Repeat([]byte{0x0D}, SizeTxid)
	pk := bytes.Repeat([]byte{0x09}, SizePubKey)

	setupOut(t, s, txid, pk, 0, 500)
	setupComputeIndex(t, s, height, txid, tweak, [][8]byte{short8(pk)})

	items, stats, err := s.FetchComputeIndexDedupTaproot(height)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || stats.SentOutputs != 1 || stats.OmittedOutputs != 0 || stats.TxsDropped != 0 {
		t.Fatalf("unexpected result: items=%d stats=%+v", len(items), stats)
	}
}
