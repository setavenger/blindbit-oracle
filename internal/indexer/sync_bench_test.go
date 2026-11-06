package indexer

// Benchmarks for the sync pipeline, (*Builder).SyncBlocks, against a fake
// Bitcoin Core REST server. They exist to compare the throughput of two
// versions of the indexer, so they only use APIs that are stable across them
// (NewBuilder, SyncBlocks, the config variables, database.DB) and are
// self-contained: every identifier in this file starts with bench/Bench.
//
//	go test -tags libsecp256k1 -run '^$' -bench SyncBlocks -benchtime 20x -count 3 ./internal/indexer/

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/rs/zerolog"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

const (
	// benchRealSyncEnd is the last height synced per op in the real-block
	// benchmark (heights 1..16, each real block served four times).
	benchRealSyncEnd = 16
	// benchTinySyncEnd is the last height synced per op in the tiny-block
	// benchmark (heights 1..200).
	benchTinySyncEnd = 200
	// benchParallelism is used for both MaxParallelRequests and
	// MaxParallelTweakComputations.
	benchParallelism = 4
)

// benchRealFixtureFiles are getblock <hash> 3 responses of mainnet blocks.
// Height h of the fake chain serves benchRealFixtureFiles[h%4].
var benchRealFixtureFiles = []string{
	"../../test_data/block_833000.json",
	"../../test_data/block_833010.json",
	"../../test_data/block_833013.json",
	"../../test_data/block_834469.json",
}

// benchPayload is a pre-serialised response body.
type benchPayload struct {
	body   []byte
	length []string // Content-Length header value
}

func newBenchPayload(body []byte) benchPayload {
	return benchPayload{body: body, length: []string{strconv.Itoa(len(body))}}
}

// benchBlock is one block as the REST interface serves it.
type benchBlock struct {
	hash  chainhash.Hash
	raw   benchPayload // /rest/block/<hash>.bin
	spent benchPayload // /rest/spenttxouts/<hash>.bin
}

// ---------------------------------------------------------------------------
// Real mainnet fixtures

type benchFixtureJSON struct {
	Result struct {
		Hash              string `json:"hash"`
		Height            int64  `json:"height"`
		Size              int    `json:"size"`
		Version           int32  `json:"version"`
		PreviousBlockHash string `json:"previousblockhash"`
		MerkleRoot        string `json:"merkleroot"`
		Time              int64  `json:"time"`
		Bits              string `json:"bits"`
		Nonce             uint32 `json:"nonce"`
		Tx                []struct {
			Txid string `json:"txid"`
			Hex  string `json:"hex"`
			Vin  []struct {
				Prevout *struct {
					Value        float64 `json:"value"`
					ScriptPubKey struct {
						Hex string `json:"hex"`
					} `json:"scriptPubKey"`
				} `json:"prevout"`
			} `json:"vin"`
			Vout []struct {
				Value float64 `json:"value"`
			} `json:"vout"`
		} `json:"tx"`
	} `json:"result"`
}

var (
	benchRealOnce   sync.Once
	benchRealBlocks []*benchBlock
	benchRealErr    error
)

// benchLoadRealBlocks parses the fixtures once per process and caches the
// serialised blocks and spent outputs.
func benchLoadRealBlocks(b *testing.B) []*benchBlock {
	b.Helper()
	benchRealOnce.Do(func() {
		for _, path := range benchRealFixtureFiles {
			blk, err := benchLoadRealBlock(path)
			if err != nil {
				benchRealErr = fmt.Errorf("%s: %w", path, err)
				return
			}
			benchRealBlocks = append(benchRealBlocks, blk)
		}
	})
	if benchRealErr != nil {
		b.Fatal(benchRealErr)
	}
	return benchRealBlocks
}

// benchBTCToSats converts a BTC amount as Bitcoin Core prints it (8 decimals)
// to satoshis. Rounding, not truncation: 0.29 BTC is 28999999.999999996 in
// float64.
func benchBTCToSats(btc float64) (int64, error) {
	sats := math.Round(btc * 1e8)
	if math.IsNaN(sats) || sats < 0 || sats > 21e6*1e8 {
		return 0, fmt.Errorf("invalid amount %v BTC", btc)
	}
	return int64(sats), nil
}

func benchLoadRealBlock(path string) (*benchBlock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var fx benchFixtureJSON
	if err := json.NewDecoder(f).Decode(&fx); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	r := fx.Result
	if len(r.Tx) == 0 {
		return nil, fmt.Errorf("no transactions")
	}

	prev, err := chainhash.NewHashFromStr(r.PreviousBlockHash)
	if err != nil {
		return nil, fmt.Errorf("previousblockhash: %w", err)
	}
	merkle, err := chainhash.NewHashFromStr(r.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("merkleroot: %w", err)
	}
	bits, err := strconv.ParseUint(r.Bits, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("bits: %w", err)
	}

	msg := wire.MsgBlock{Header: wire.BlockHeader{
		Version:    r.Version,
		PrevBlock:  *prev,
		MerkleRoot: *merkle,
		Timestamp:  time.Unix(r.Time, 0),
		Bits:       uint32(bits),
		Nonce:      r.Nonce,
	}}

	// Spent outputs in the format ParseSpentTxOuts reads: varint nTx, then per
	// tx varint k (coinbase: 0) and k times int64 LE value + varbytes script.
	var spent bytes.Buffer
	if err := wire.WriteVarInt(&spent, 0, uint64(len(r.Tx))); err != nil {
		return nil, err
	}
	want := make([][]*wire.TxOut, len(r.Tx))
	var inSats, outSats, coinbaseSats int64

	for i, jtx := range r.Tx {
		rawTx, err := hex.DecodeString(jtx.Hex)
		if err != nil {
			return nil, fmt.Errorf("tx %d hex: %w", i, err)
		}
		var tx wire.MsgTx
		if err := tx.Deserialize(bytes.NewReader(rawTx)); err != nil {
			return nil, fmt.Errorf("tx %d deserialise: %w", i, err)
		}
		if got := tx.TxHash().String(); got != jtx.Txid {
			return nil, fmt.Errorf("tx %d: txid %s, fixture says %s", i, got, jtx.Txid)
		}
		if len(jtx.Vin) != len(tx.TxIn) {
			return nil, fmt.Errorf("tx %d: %d vin in JSON, %d in hex", i, len(jtx.Vin), len(tx.TxIn))
		}
		msg.Transactions = append(msg.Transactions, &tx)

		for _, out := range jtx.Vout {
			sats, err := benchBTCToSats(out.Value)
			if err != nil {
				return nil, fmt.Errorf("tx %d vout: %w", i, err)
			}
			if i == 0 {
				coinbaseSats += sats
			} else {
				outSats += sats
			}
		}

		var prevouts []*wire.TxOut
		for j, vin := range jtx.Vin {
			if vin.Prevout == nil {
				continue
			}
			sats, err := benchBTCToSats(vin.Prevout.Value)
			if err != nil {
				return nil, fmt.Errorf("tx %d vin %d: %w", i, j, err)
			}
			script, err := hex.DecodeString(vin.Prevout.ScriptPubKey.Hex)
			if err != nil {
				return nil, fmt.Errorf("tx %d vin %d script: %w", i, j, err)
			}
			prevouts = append(prevouts, &wire.TxOut{Value: sats, PkScript: script})
			inSats += sats
		}
		// The indexer pairs TxIn[j] with prevout j, so every non-coinbase
		// input needs one.
		if i == 0 && len(prevouts) != 0 {
			return nil, fmt.Errorf("coinbase has %d prevouts", len(prevouts))
		}
		if i > 0 && len(prevouts) != len(tx.TxIn) {
			return nil, fmt.Errorf("tx %d: %d prevouts for %d inputs", i, len(prevouts), len(tx.TxIn))
		}
		want[i] = prevouts

		if err := wire.WriteVarInt(&spent, 0, uint64(len(prevouts))); err != nil {
			return nil, err
		}
		for _, p := range prevouts {
			if err := binary.Write(&spent, binary.LittleEndian, p.Value); err != nil {
				return nil, err
			}
			if err := wire.WriteVarBytes(&spent, 0, p.PkScript); err != nil {
				return nil, err
			}
		}
	}

	if got := msg.BlockHash().String(); got != r.Hash {
		return nil, fmt.Errorf("block hash %s, fixture says %s", got, r.Hash)
	}

	// Every amount went through benchBTCToSats: the coinbase of these blocks
	// claims exactly subsidy + fees, so any conversion error shows up here.
	subsidy := int64(50_0000_0000) >> uint(r.Height/210_000)
	if fees := inSats - outSats; coinbaseSats != subsidy+fees {
		return nil, fmt.Errorf("coinbase pays %d sats, subsidy+fees is %d", coinbaseSats, subsidy+fees)
	}

	var raw bytes.Buffer
	if err := msg.Serialize(&raw); err != nil {
		return nil, fmt.Errorf("serialise block: %w", err)
	}
	if raw.Len() != r.Size {
		return nil, fmt.Errorf("serialised block is %d bytes, fixture says %d", raw.Len(), r.Size)
	}

	// Round-trip the spent outputs through the indexer's own parser.
	spentReader := bytes.NewReader(spent.Bytes())
	parsed, err := ParseSpentTxOuts(spentReader)
	if err != nil {
		return nil, fmt.Errorf("parse spent outputs: %w", err)
	}
	if spentReader.Len() != 0 {
		return nil, fmt.Errorf("spent outputs: %d trailing bytes", spentReader.Len())
	}
	if len(parsed) != len(msg.Transactions) {
		return nil, fmt.Errorf("spent outputs: %d txs, block has %d", len(parsed), len(msg.Transactions))
	}
	for i := range parsed {
		if len(parsed[i]) != len(want[i]) {
			return nil, fmt.Errorf("spent outputs tx %d: %d prevouts, want %d", i, len(parsed[i]), len(want[i]))
		}
		for j := range parsed[i] {
			if parsed[i][j].Value != want[i][j].Value || !bytes.Equal(parsed[i][j].PkScript, want[i][j].PkScript) {
				return nil, fmt.Errorf("spent outputs tx %d vin %d: round-trip mismatch", i, j)
			}
		}
	}

	return &benchBlock{
		hash:  msg.BlockHash(),
		raw:   newBenchPayload(raw.Bytes()),
		spent: newBenchPayload(spent.Bytes()),
	}, nil
}

// ---------------------------------------------------------------------------
// Tiny coinbase-only chain

var (
	benchTinyOnce   sync.Once
	benchTinyBlocks []*benchBlock
)

// benchLoadTinyBlocks returns a chain of coinbase-only blocks for heights
// 0..benchTinySyncEnd. Their processing cost is negligible, so a sync of them
// is dominated by the HTTP request path.
func benchLoadTinyBlocks(b *testing.B) []*benchBlock {
	b.Helper()
	benchTinyOnce.Do(func() {
		var spent bytes.Buffer
		_ = wire.WriteVarInt(&spent, 0, 1) // one tx
		_ = wire.WriteVarInt(&spent, 0, 0) // the coinbase spends nothing
		spentPayload := newBenchPayload(spent.Bytes())

		prev := chainhash.Hash{}
		for h := 0; h <= benchTinySyncEnd; h++ {
			tx := wire.NewMsgTx(2)
			tx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
				SignatureScript:  []byte(fmt.Sprintf("bench/%d", h)),
				Sequence:         wire.MaxTxInSequenceNum,
			})
			tx.AddTxOut(&wire.TxOut{Value: 50_0000_0000, PkScript: []byte{0x51}}) // OP_TRUE, not taproot

			merkle := tx.TxHash()
			msg := wire.NewMsgBlock(wire.NewBlockHeader(1, &prev, &merkle, 0x207fffff, uint32(h)))
			msg.Header.Timestamp = time.Unix(1_700_000_000+int64(h)*600, 0)
			_ = msg.AddTransaction(tx)

			var raw bytes.Buffer
			if err := msg.Serialize(&raw); err != nil {
				panic(err)
			}
			prev = msg.BlockHash()
			benchTinyBlocks = append(benchTinyBlocks, &benchBlock{
				hash:  prev,
				raw:   newBenchPayload(raw.Bytes()),
				spent: spentPayload,
			})
		}
	})
	return benchTinyBlocks
}

// ---------------------------------------------------------------------------
// Fake REST server

var (
	benchOctetStream = []string{"application/octet-stream"}
	benchJSONType    = []string{"application/json"}
	benchLength32    = []string{"32"}
)

// benchNode serves the REST endpoints the indexer uses from pre-serialised
// bodies, so a request costs no serialisation on the server side.
type benchNode struct {
	srv       *httptest.Server
	hashes    [][]byte               // height -> 32 raw bytes, internal byte order
	blocks    map[string]*benchBlock // key: hash in RPC (reversed hex) form
	chainInfo benchPayload
	requests  atomic.Int64
}

func newBenchNode(b *testing.B, tip int64, blockAt func(height int64) *benchBlock) *benchNode {
	b.Helper()
	n := &benchNode{
		hashes: make([][]byte, tip+1),
		blocks: map[string]*benchBlock{},
	}
	for h := int64(0); h <= tip; h++ {
		blk := blockAt(h)
		n.hashes[h] = blk.hash[:]
		n.blocks[blk.hash.String()] = blk
	}
	info, err := json.Marshal(map[string]any{
		"chain":         "main",
		"blocks":        tip,
		"headers":       tip,
		"bestblockhash": blockAt(tip).hash.String(),
	})
	if err != nil {
		b.Fatal(err)
	}
	n.chainInfo = newBenchPayload(info)
	n.srv = httptest.NewServer(n)
	b.Cleanup(n.srv.Close)
	return n
}

func benchWrite(w http.ResponseWriter, contentType []string, p benchPayload) {
	h := w.Header()
	h["Content-Type"] = contentType
	h["Content-Length"] = p.length
	_, _ = w.Write(p.body)
}

func (n *benchNode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n.requests.Add(1)
	path, ok := strings.CutPrefix(r.URL.Path, "/rest/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if path == "chaininfo.json" {
		benchWrite(w, benchJSONType, n.chainInfo)
		return
	}
	endpoint, arg, _ := strings.Cut(path, "/")
	arg, ok = strings.CutSuffix(arg, ".bin")
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch endpoint {
	case "blockhashbyheight":
		h, err := strconv.ParseInt(arg, 10, 64)
		if err != nil || h < 0 || h >= int64(len(n.hashes)) {
			http.NotFound(w, r)
			return
		}
		benchWrite(w, benchOctetStream, benchPayload{body: n.hashes[h], length: benchLength32})
	case "block", "spenttxouts":
		blk := n.blocks[arg]
		if blk == nil {
			http.NotFound(w, r)
			return
		}
		if endpoint == "block" {
			benchWrite(w, benchOctetStream, blk.raw)
		} else {
			benchWrite(w, benchOctetStream, blk.spent)
		}
	default:
		http.NotFound(w, r)
	}
}

// ---------------------------------------------------------------------------
// Discard store

// benchStore is a database.DB that only counts what it is given. The embedded
// nil interface satisfies the methods SyncBlocks does not use; calling one of
// them panics, which makes an unexpected dependency obvious.
type benchStore struct {
	database.DB

	mu        sync.Mutex
	tipHash   []byte
	tipHeight uint32
	blocks    int64
	txs       int64 // txs of interest (non-nil entries)
	tweaks    int64
}

func (s *benchStore) ApplyBlock(blk *database.DBBlock) error {
	var txs, tweaks int64
	for _, tx := range blk.Txs {
		if tx == nil {
			continue
		}
		txs++
		if tx.Tweak != nil {
			tweaks++
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks++
	s.txs += txs
	s.tweaks += tweaks
	if blk.Height >= s.tipHeight {
		s.tipHeight = blk.Height
		s.tipHash = append(s.tipHash[:0], blk.Hash[:]...)
	}
	return nil
}

func (s *benchStore) FlushBatch(bool) error { return nil }

func (s *benchStore) BatchSize() int { return 0 }

func (s *benchStore) GetChainTip() ([]byte, uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.tipHash), s.tipHeight, nil
}

func (s *benchStore) counts() (blocks, txs, tweaks int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocks, s.txs, s.tweaks
}

// ---------------------------------------------------------------------------
// Harness

// benchSetup points the indexer at endpoint with the benchmark's parallelism
// and silences logging; everything is restored when the benchmark ends.
func benchSetup(b *testing.B, endpoint string) {
	b.Helper()
	oldEndpoint := config.RestEndpoint
	oldRequests := config.MaxParallelRequests
	oldTweaks := config.MaxParallelTweakComputations
	oldLogger := logging.L

	config.RestEndpoint = endpoint
	config.MaxParallelRequests = benchParallelism
	config.MaxParallelTweakComputations = benchParallelism
	// Per-block trace/info logging would otherwise dominate the profile.
	logging.L = logging.L.Level(zerolog.Disabled)

	b.Cleanup(func() {
		config.RestEndpoint = oldEndpoint
		config.MaxParallelRequests = oldRequests
		config.MaxParallelTweakComputations = oldTweaks
		logging.L = oldLogger
	})
}

// benchSyncBlocks runs SyncBlocks(ctx, 1, end) per op against node and
// reports blocks/s and requests per op.
func benchSyncBlocks(b *testing.B, node *benchNode, end int64, wantTweaks bool) {
	b.Helper()
	benchSetup(b, node.srv.URL)

	ctx := context.Background()
	store := &benchStore{}
	builder := NewBuilder(ctx, store)

	// Untimed warm-up: opens the keep-alive connections and checks that a
	// sync indexes every block.
	if err := builder.SyncBlocks(ctx, 1, end); err != nil {
		b.Fatalf("warm-up sync: %v", err)
	}
	blocks, txs, tweaks := store.counts()
	if blocks != end {
		b.Fatalf("warm-up sync applied %d blocks, want %d", blocks, end)
	}
	if wantTweaks && tweaks == 0 {
		b.Fatalf("warm-up sync computed no tweaks (%d txs of interest)", txs)
	}

	node.requests.Store(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := builder.SyncBlocks(ctx, 1, end); err != nil {
			b.Fatalf("sync: %v", err)
		}
	}
	b.StopTimer()

	if got, _, _ := store.counts(); got != end*int64(b.N+1) {
		b.Fatalf("applied %d blocks, want %d", got, end*int64(b.N+1))
	}
	b.ReportMetric(float64(end)*float64(b.N)/b.Elapsed().Seconds(), "blocks/s")
	b.ReportMetric(float64(node.requests.Load())/float64(b.N), "req/op")
}

// BenchmarkSyncBlocksRealBlocks syncs 16 heights of real mainnet blocks
// (four fixtures, ~2 MB and ~800-3000 txs each) per op.
func BenchmarkSyncBlocksRealBlocks(b *testing.B) {
	blocks := benchLoadRealBlocks(b)
	node := newBenchNode(b, benchRealSyncEnd, func(h int64) *benchBlock {
		return blocks[h%int64(len(blocks))]
	})
	benchSyncBlocks(b, node, benchRealSyncEnd, true)
}

// BenchmarkSyncBlocksTinyBlocks syncs 200 coinbase-only blocks per op, so the
// time is dominated by the HTTP request path: the worst case for any
// per-request overhead.
func BenchmarkSyncBlocksTinyBlocks(b *testing.B) {
	blocks := benchLoadTinyBlocks(b)
	node := newBenchNode(b, benchTinySyncEnd, func(h int64) *benchBlock {
		return blocks[h]
	})
	benchSyncBlocks(b, node, benchTinySyncEnd, false)
}
