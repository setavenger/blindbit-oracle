package server

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/gin-gonic/gin"

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

func reversedHex(b []byte) string {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return hex.EncodeToString(out)
}

// servedOutpoint is how /full-block encodes one spent outpoint in `inputs`:
// the stored prev txid byte-reversed, followed by the vout as stored.
func servedOutpoint(prevTxid []byte, vout uint32) string {
	return reversedHex(prevTxid) + hex.EncodeToString(binary.BigEndian.AppendUint32(nil, vout))
}

type fullBlockJSON struct {
	Index []struct {
		TxId   string   `json:"txid"`
		Tweak  string   `json:"tweak"`
		Inputs []string `json:"inputs"`
		UTXOs  []struct {
			Vout   uint32 `json:"vout"`
			Amount uint64 `json:"amount"`
			Pubkey string `json:"pubkey"`
		} `json:"utxos"`
	} `json:"index"`
}

// Regression test for #58: transactions which spend tracked outputs but have
// no indexed outputs of their own were dropped from /full-block.
func TestGetFullBlockIncludesInputOnlyTxs(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	store := dbpebble.NewStore(db)
	defer store.Close()

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

	block := &database.DBBlock{Height: 100, Hash: &blockhash, Txs: []*database.Tx{
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

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/full-block/:blockheight", NewHandler(store).GetFullBlock)

	fetch := func() (string, fullBlockJSON) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/full-block/100", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var parsed fullBlockJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("bad json: %v: %s", err, rec.Body.String())
		}
		return rec.Body.String(), parsed
	}

	body, resp := fetch()
	t.Logf("response body: %s", body)

	// tweakless items must not be padded with zeros, and have a list for utxos
	if strings.Contains(body, strings.Repeat("00", 33)) {
		t.Errorf("response contains a zero-filled tweak")
	}
	if strings.Contains(body, `"utxos":null`) {
		t.Errorf("response contains null utxos, want an empty list")
	}

	seen := make(map[string]int)
	for i, item := range resp.Index {
		if _, dup := seen[item.TxId]; dup {
			t.Errorf("txid %s served more than once", item.TxId)
		}
		seen[item.TxId] = i
	}
	if len(resp.Index) != 3 {
		t.Errorf("want 3 items, got %d", len(resp.Index))
	}

	// tx A: has a tweak, must be served exactly as before
	if i, ok := seen[reversedHex(txA)]; !ok {
		t.Errorf("tx A (tweak + outputs) missing")
	} else {
		item := resp.Index[i]
		if want := hex.EncodeToString(tweakA[:]); item.Tweak != want {
			t.Errorf("tx A tweak: want %s, got %s", want, item.Tweak)
		}
		if want := servedOutpoint(prevA, 3); len(item.Inputs) != 1 || item.Inputs[0] != want {
			t.Errorf("tx A inputs: want [%s], got %v", want, item.Inputs)
		}
		if len(item.UTXOs) != 1 {
			t.Errorf("tx A utxos: want 1, got %d", len(item.UTXOs))
		} else if u := item.UTXOs[0]; u.Vout != 1 || u.Amount != 5000 || u.Pubkey != hex.EncodeToString(pubkeyA) {
			t.Errorf("tx A utxo mismatch: %+v", u)
		}
	}

	// tx B: input-only, the reason for #58
	if i, ok := seen[reversedHex(txB)]; !ok {
		t.Errorf("tx B (input-only) missing")
	} else {
		item := resp.Index[i]
		if item.Tweak != "" {
			t.Errorf("tx B tweak: want empty string, got %q", item.Tweak)
		}
		want := []string{servedOutpoint(prevB1, 7), servedOutpoint(prevB2, 258)}
		if len(item.Inputs) != 2 || item.Inputs[0] != want[0] || item.Inputs[1] != want[1] {
			t.Errorf("tx B inputs: want %v, got %v", want, item.Inputs)
		}
		if len(item.UTXOs) != 0 {
			t.Errorf("tx B utxos: want none, got %d", len(item.UTXOs))
		}
	}

	// tx C: no tweak, so its outputs were never indexed
	if i, ok := seen[reversedHex(txC)]; !ok {
		t.Errorf("tx C (tweakless) missing")
	} else {
		item := resp.Index[i]
		if item.Tweak != "" {
			t.Errorf("tx C tweak: want empty string, got %q", item.Tweak)
		}
		if want := servedOutpoint(prevC, 0); len(item.Inputs) != 1 || item.Inputs[0] != want {
			t.Errorf("tx C inputs: want [%s], got %v", want, item.Inputs)
		}
		if len(item.UTXOs) != 0 {
			t.Errorf("tx C utxos: want none, got %d", len(item.UTXOs))
		}
	}

	// ordering is by stored txid over the union of both sources
	wantOrder := []string{reversedHex(txB), reversedHex(txA), reversedHex(txC)}
	if len(resp.Index) == len(wantOrder) {
		for i := range wantOrder {
			if resp.Index[i].TxId != wantOrder[i] {
				t.Errorf("item %d: want txid %s, got %s", i, wantOrder[i], resp.Index[i].TxId)
			}
		}
	}

	// determinism
	if body2, _ := fetch(); body2 != body {
		t.Errorf("/full-block is not deterministic:\nfirst:  %s\nsecond: %s", body, body2)
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

	var blockhash chainhash.Hash
	copy(blockhash[:], seq(0x01, 32))

	txid, prev1, prev2 := seq(0x10, 32), seq(0x31, 32), seq(0x41, 32)
	block := &database.DBBlock{Height: 101, Hash: &blockhash, Txs: []*database.Tx{{
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

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/full-block/:blockheight", NewHandler(store).GetFullBlock)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/full-block/101", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	t.Logf("response body: %s", body)

	var resp fullBlockJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v: %s", err, body)
	}
	if len(resp.Index) != 1 {
		t.Fatalf("want exactly 1 item, got %d", len(resp.Index))
	}

	// the raw encoding matters to clients: an empty string and an empty list
	for _, want := range []string{`"tweak":""`, `"utxos":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("response does not contain %s", want)
		}
	}

	item := resp.Index[0]
	if want := reversedHex(txid); item.TxId != want {
		t.Errorf("txid: want %s, got %s", want, item.TxId)
	}
	if item.Tweak != "" {
		t.Errorf("tweak: want empty string, got %q", item.Tweak)
	}
	if len(item.UTXOs) != 0 {
		t.Errorf("utxos: want none, got %d", len(item.UTXOs))
	}
	want := []string{servedOutpoint(prev1, 7), servedOutpoint(prev2, 258)}
	if len(item.Inputs) != 2 || item.Inputs[0] != want[0] || item.Inputs[1] != want[1] {
		t.Errorf("inputs: want %v, got %v", want, item.Inputs)
	}
}
