package dbpebble

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// newTestStore builds a store on a throwaway pebble db holding one indexed
// block at height 100 with two silent payment transactions, and a second block
// at height 101 that spends txA vout 1 and txB vout 0.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := pebble.Open(t.TempDir(), &pebble.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)

	pk := func(b byte) []byte {
		p := make([]byte, SizePubKey)
		p[0] = b
		return p
	}
	txid := func(b byte) []byte {
		id := make([]byte, SizeTxid)
		id[0] = b
		return id
	}
	tweak := func(b byte) *[33]byte {
		var tw [33]byte
		tw[0] = b
		return &tw
	}

	txA, txB := txid(0xaa), txid(0xbb)

	err = s.ApplyBlock(&database.DBBlock{
		Height: 100,
		Hash:   &chainhash.Hash{0x01},
		Txs: []*database.Tx{
			{
				Txid:  txA,
				Tweak: tweak(0x11),
				Outs: []*database.Output{
					{Txid: txA, Vout: 0, Amount: 500, Pubkey: pk(0xa0)},
					{Txid: txA, Vout: 1, Amount: 1500, Pubkey: pk(0xa1)},
					{Txid: txA, Vout: 2, Amount: 2500, Pubkey: pk(0xa2)},
				},
			},
			{
				Txid:  txB,
				Tweak: tweak(0x22),
				Outs: []*database.Output{
					{Txid: txB, Vout: 0, Amount: 100, Pubkey: pk(0xb0)},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.ApplyBlock(&database.DBBlock{
		Height: 101,
		Hash:   &chainhash.Hash{0x02},
		Txs: []*database.Tx{
			{
				Txid: txid(0xcc),
				Ins: []*database.In{
					{PrevTxid: txA, PrevVout: 1, Pubkey: pk(0xa1)},
					{PrevTxid: txB, PrevVout: 0, Pubkey: pk(0xb0)},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FlushBatch(true); err != nil {
		t.Fatal(err)
	}

	return s
}

func TestFetchComputeIndexFiltered(t *testing.T) {
	s := newTestStore(t)
	_, tip, err := s.GetChainTip()
	if err != nil {
		t.Fatal(err)
	}

	// one 8 byte prefix per output, in vout order
	short := func(bs ...byte) []byte {
		var out []byte
		for _, b := range bs {
			out = append(out, append([]byte{b}, make([]byte, 7)...)...)
		}
		return out
	}

	cases := []struct {
		name       string
		dustLimit  uint64
		cutThrough bool
		want       [][]byte // outputs_short per surviving entry, txA before txB
	}{
		{
			"unfiltered", 0, false,
			[][]byte{short(0xa0, 0xa1, 0xa2), short(0xb0)},
		},
		{
			// txB's only output is 100, so txB goes and txA stays whole
			"dust", 1000, false,
			[][]byte{short(0xa0, 0xa1, 0xa2)},
		},
		{
			// txA vout 1 is spent but vout 0 and 2 are not, so txA stays whole
			"cut through", 0, true,
			[][]byte{short(0xa0, 0xa1, 0xa2)},
		},
		{
			// vout 2 is unspent and clears the limit, so txA stays whole
			"dust and cut through", 1000, true,
			[][]byte{short(0xa0, 0xa1, 0xa2)},
		},
		{
			"dust above every output", 3000, false,
			nil,
		},
		{
			"dust above every unspent output", 3000, true,
			nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.FetchComputeIndexFiltered(100, tip, c.dustLimit, c.cutThrough)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(c.want))
			}
			for i := range got {
				if !bytes.Equal(got[i].OutputsShort, c.want[i]) {
					t.Errorf("entry %d outputs_short = %x, want %x",
						i, got[i].OutputsShort, c.want[i])
				}
			}
		})
	}
}

// the unfiltered request must return exactly what the stored index holds
func TestFetchComputeIndexFilteredPassthrough(t *testing.T) {
	s := newTestStore(t)
	_, tip, err := s.GetChainTip()
	if err != nil {
		t.Fatal(err)
	}

	stored, err := s.FetchComputeIndex(100)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.FetchComputeIndexFiltered(100, tip, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(stored) {
		t.Fatalf("got %d entries, want %d", len(got), len(stored))
	}
	for i := range got {
		if !bytes.Equal(got[i].Txid, stored[i].Txid) ||
			!bytes.Equal(got[i].Tweak, stored[i].Tweak) ||
			!bytes.Equal(got[i].OutputsShort, stored[i].OutputsShort) {
			t.Fatalf("entry %d differs from the stored index", i)
		}
	}
}

// A kept tx must carry all of its outputs: a missing k stops the scanner.
func TestFetchComputeIndexFilteredKeepsTxWhole(t *testing.T) {
	s := newTestStore(t)
	_, tip, err := s.GetChainTip()
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name       string
		dustLimit  uint64
		cutThrough bool
	}{
		{"cut through", 0, true},
		{"dust and cut through", 1000, true},
		{"dust", 1000, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.FetchComputeIndexFiltered(100, tip, c.dustLimit, c.cutThrough)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 {
				t.Fatal("txA was dropped entirely")
			}
			if n := len(got[0].OutputsShort) / 8; n != 3 {
				t.Fatalf("txA came back with %d of 3 outputs; a scanner would stop at the first gap", n)
			}
		})
	}
}
