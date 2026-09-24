package v2

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"google.golang.org/protobuf/proto"

	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// seq returns n bytes counting up from start. The values are deliberately not
// palindromic so that a byte-order mistake changes the result.
func seq(start byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func reversed(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}

// servedOutpoint is how GetFullBlock encodes one spent outpoint in `inputs`:
// the stored prev txid byte-reversed, followed by the vout as stored.
func servedOutpoint(prevTxid []byte, vout uint32) []byte {
	out := reversed(prevTxid)
	return binary.BigEndian.AppendUint32(out, vout)
}

// Regression test for #58: transactions which spend tracked outputs but have
// no indexed outputs of their own were dropped from GetFullBlock.
func TestGetFullBlockIncludesInputOnlyTxs(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	store := dbpebble.NewStore(db)
	defer store.Close()

	const height = 100
	var blockhash chainhash.Hash
	copy(blockhash[:], seq(0x01, 32))

	var tweakA [33]byte
	copy(tweakA[:], seq(0x02, 33))

	// txids are chosen so that the input-only tx sorts first and the tweakless
	// tx sorts last; the tx with a tweak sits in between.
	txA := seq(0x80, 32) // tweak + output + taproot input
	txB := seq(0x10, 32) // input-only: spends tracked outputs, creates none
	txC := seq(0xC0, 32) // taproot input and output, but no tweak

	prevA, prevB1, prevB2, prevC := seq(0x21, 32), seq(0x31, 32), seq(0x41, 32), seq(0x51, 32)
	pubkeyA := seq(0x61, 32)

	block := &database.DBBlock{Height: height, Hash: &blockhash, Txs: []*database.Tx{
		{
			Txid: txA, Tweak: &tweakA,
			Outs: []*database.Output{{Txid: txA, Vout: 1, Amount: 5000, Pubkey: pubkeyA}},
			Ins:  []*database.In{{PrevTxid: prevA, PrevVout: 3, Pubkey: seq(0x71, 32)}},
		},
		{
			Txid: txB,
			Ins: []*database.In{
				{PrevTxid: prevB1, PrevVout: 7, Pubkey: seq(0x72, 32)},
				{PrevTxid: prevB2, PrevVout: 258, Pubkey: seq(0x73, 32)},
			},
		},
		{
			Txid: txC,
			Outs: []*database.Output{{Txid: txC, Vout: 0, Amount: 9000, Pubkey: seq(0x62, 32)}},
			Ins:  []*database.In{{PrevTxid: prevC, PrevVout: 0, Pubkey: seq(0x74, 32)}},
		},
	}}
	if err := store.ApplyBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := store.FlushBatch(true); err != nil {
		t.Fatal(err)
	}

	svc := NewOracleService(store)
	resp, err := svc.GetFullBlock(context.Background(), &pb.BlockHeightRequest{BlockHeight: height})
	if err != nil {
		t.Fatal(err)
	}

	items := make(map[string]*pb.FullTxItem)
	for _, item := range resp.Index {
		if _, dup := items[string(item.Txid)]; dup {
			t.Errorf("txid %x served more than once", item.Txid)
		}
		items[string(item.Txid)] = item
	}
	if len(resp.Index) != 3 {
		t.Errorf("want 3 items, got %d", len(resp.Index))
	}

	// tx A: has a tweak, must be served exactly as before
	if item, ok := items[string(reversed(txA))]; !ok {
		t.Errorf("tx A (tweak + outputs) missing")
	} else {
		if !bytes.Equal(item.Tweak, tweakA[:]) {
			t.Errorf("tx A tweak: want %x, got %x", tweakA, item.Tweak)
		}
		if want := servedOutpoint(prevA, 3); !bytes.Equal(item.Inputs, want) {
			t.Errorf("tx A inputs: want %x, got %x", want, item.Inputs)
		}
		if len(item.Utxos) != 1 {
			t.Errorf("tx A utxos: want 1, got %d", len(item.Utxos))
		} else if u := item.Utxos[0]; u.Vout != 1 || u.Amount != 5000 || !bytes.Equal(u.Pubkey, pubkeyA) {
			t.Errorf("tx A utxo mismatch: %v", u)
		}
	}

	// tx B: input-only, the reason for #58
	if item, ok := items[string(reversed(txB))]; !ok {
		t.Errorf("tx B (input-only) missing")
	} else {
		if len(item.Tweak) != 0 {
			t.Errorf("tx B tweak: want empty, got %x", item.Tweak)
		}
		want := append(servedOutpoint(prevB1, 7), servedOutpoint(prevB2, 258)...)
		if !bytes.Equal(item.Inputs, want) {
			t.Errorf("tx B inputs: want %x, got %x", want, item.Inputs)
		}
		if len(item.Utxos) != 0 {
			t.Errorf("tx B utxos: want none, got %d", len(item.Utxos))
		}
	}

	// tx C: no tweak, so its outputs were never indexed
	if item, ok := items[string(reversed(txC))]; !ok {
		t.Errorf("tx C (tweakless) missing")
	} else {
		if len(item.Tweak) != 0 {
			t.Errorf("tx C tweak: want empty, got %x", item.Tweak)
		}
		if want := servedOutpoint(prevC, 0); !bytes.Equal(item.Inputs, want) {
			t.Errorf("tx C inputs: want %x, got %x", want, item.Inputs)
		}
		if len(item.Utxos) != 0 {
			t.Errorf("tx C utxos: want none, got %d", len(item.Utxos))
		}
	}

	// ordering is by stored txid over the union of both sources
	wantOrder := [][]byte{reversed(txB), reversed(txA), reversed(txC)}
	if len(resp.Index) == len(wantOrder) {
		for i := range wantOrder {
			if !bytes.Equal(resp.Index[i].Txid, wantOrder[i]) {
				t.Errorf("item %d: want txid %x, got %x", i, wantOrder[i], resp.Index[i].Txid)
			}
		}
	}

	// determinism
	resp2, err := svc.GetFullBlock(context.Background(), &pb.BlockHeightRequest{BlockHeight: height})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(resp, resp2) {
		t.Errorf("GetFullBlock is not deterministic:\nfirst:  %v\nsecond: %v", resp, resp2)
	}
}

// Regression test for #58: a block whose only relevant transaction is
// input-only. No transaction in the block has outputs or a tweak, so the
// outputs index is empty and the item can only come from the txid-outpoints
// index.
func TestGetFullBlockOnlyInputOnlyTx(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	store := dbpebble.NewStore(db)
	defer store.Close()

	const height = 101
	var blockhash chainhash.Hash
	copy(blockhash[:], seq(0x01, 32))

	txid, prev1, prev2 := seq(0x10, 32), seq(0x31, 32), seq(0x41, 32)
	block := &database.DBBlock{Height: height, Hash: &blockhash, Txs: []*database.Tx{{
		Txid: txid,
		Ins: []*database.In{
			{PrevTxid: prev1, PrevVout: 7, Pubkey: seq(0x72, 32)},
			{PrevTxid: prev2, PrevVout: 258, Pubkey: seq(0x73, 32)},
		},
	}}}
	if err := store.ApplyBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := store.FlushBatch(true); err != nil {
		t.Fatal(err)
	}

	resp, err := NewOracleService(store).GetFullBlock(
		context.Background(), &pb.BlockHeightRequest{BlockHeight: height},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Index) != 1 {
		t.Fatalf("want exactly 1 item, got %d", len(resp.Index))
	}

	item := resp.Index[0]
	if want := reversed(txid); !bytes.Equal(item.Txid, want) {
		t.Errorf("txid: want %x, got %x", want, item.Txid)
	}
	if len(item.Tweak) != 0 {
		t.Errorf("tweak: want empty, got %x", item.Tweak)
	}
	if len(item.Utxos) != 0 {
		t.Errorf("utxos: want none, got %d", len(item.Utxos))
	}
	want := append(servedOutpoint(prev1, 7), servedOutpoint(prev2, 258)...)
	if !bytes.Equal(item.Inputs, want) {
		t.Errorf("inputs: want %x, got %x", want, item.Inputs)
	}
}
