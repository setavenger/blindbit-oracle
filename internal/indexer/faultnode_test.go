package indexer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// faultKind selects how the fake node misbehaves for one request.
type faultKind int

const (
	// faultDrop closes the connection without a response: the client sees a
	// transport error, like the timeouts that triggered the original deadlock.
	faultDrop faultKind = iota + 1
	// faultHang never answers until the request is cancelled or the test ends.
	faultHang
)

// faultNode serves the subset of Bitcoin Core's REST interface used by the
// indexer (chaininfo, blockhashbyheight, block, spenttxouts) from a chain of
// coinbase-only blocks, and can inject faults per endpoint and height.
type faultNode struct {
	t       *testing.T
	srv     *httptest.Server
	blocks  []*wire.MsgBlock
	heights map[chainhash.Hash]int

	mu     sync.Mutex
	faults map[string]faultKind // key: "<endpoint>/<height>"
	stop   chan struct{}
}

func newFaultNode(t *testing.T, tip int) *faultNode {
	t.Helper()
	n := &faultNode{
		t:       t,
		heights: map[chainhash.Hash]int{},
		faults:  map[string]faultKind{},
		stop:    make(chan struct{}),
	}
	prev := chainhash.Hash{}
	for h := 0; h <= tip; h++ {
		blk := faultCoinbaseBlock(prev, h)
		n.blocks = append(n.blocks, blk)
		prev = blk.BlockHash()
		n.heights[prev] = h
	}
	n.srv = httptest.NewServer(http.HandlerFunc(n.serve))

	oldEndpoint := config.RestEndpoint
	config.RestEndpoint = n.srv.URL
	t.Cleanup(func() {
		close(n.stop) // release hanging handlers before closing the server
		n.srv.CloseClientConnections()
		n.srv.Close()
		config.RestEndpoint = oldEndpoint
	})
	return n
}

func faultCoinbaseBlock(prev chainhash.Hash, height int) *wire.MsgBlock {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte(fmt.Sprintf("faultnode/%d", height)),
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 50_0000_0000, PkScript: []byte{0x51}})

	blk := wire.NewMsgBlock(wire.NewBlockHeader(1, &prev, &chainhash.Hash{}, 0x207fffff, uint32(height)))
	blk.Header.Timestamp = time.Unix(1_700_000_000+int64(height)*600, 0)
	blk.AddTransaction(tx)
	return blk
}

func (n *faultNode) inject(endpoint string, height int, kind faultKind) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.faults[fmt.Sprintf("%s/%d", endpoint, height)] = kind
}

func (n *faultNode) fault(endpoint string, height int) faultKind {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.faults[fmt.Sprintf("%s/%d", endpoint, height)]
}

func (n *faultNode) hashAt(height int) chainhash.Hash { return n.blocks[height].BlockHash() }

func (n *faultNode) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/rest/")
	endpoint, arg, _ := strings.Cut(path, "/")

	if path == "chaininfo.json" {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain":  "regtest",
			"blocks": len(n.blocks) - 1,
		})
		return
	}

	arg = strings.TrimSuffix(arg, ".bin")
	height := -1
	switch endpoint {
	case "blockhashbyheight":
		h, err := strconv.Atoi(arg)
		if err != nil || h < 0 || h >= len(n.blocks) {
			http.NotFound(w, r)
			return
		}
		height = h
	case "block", "spenttxouts":
		hash, err := chainhash.NewHashFromStr(arg)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		h, ok := n.heights[*hash]
		if !ok {
			http.NotFound(w, r)
			return
		}
		height = h
	default:
		http.NotFound(w, r)
		return
	}

	switch n.fault(endpoint, height) {
	case faultDrop:
		hj, ok := w.(http.Hijacker)
		if !ok {
			n.t.Errorf("faultnode: response writer cannot hijack")
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	case faultHang:
		select {
		case <-r.Context().Done():
		case <-n.stop:
		}
		return
	}

	switch endpoint {
	case "blockhashbyheight":
		hash := n.hashAt(height)
		_, _ = w.Write(hash[:])
	case "block":
		var buf bytes.Buffer
		if err := n.blocks[height].Serialize(&buf); err != nil {
			n.t.Errorf("faultnode: serialize block %d: %v", height, err)
			return
		}
		_, _ = w.Write(buf.Bytes())
	case "spenttxouts":
		// one transaction (the coinbase) spending zero outputs
		var buf bytes.Buffer
		_ = wire.WriteVarInt(&buf, 0, uint64(len(n.blocks[height].Transactions)))
		for range n.blocks[height].Transactions {
			_ = wire.WriteVarInt(&buf, 0, 0)
		}
		_, _ = w.Write(buf.Bytes())
	}
}

// faultStore is an in-memory database.DB with an optional write failure.
// Only the methods used by the indexer's sync paths are implemented; calling
// any other method panics on the nil embedded interface.
type faultStore struct {
	database.DB

	mu       sync.Mutex
	byHeight map[uint32][]byte
	applied  map[uint32]int
	failAt   map[uint32]error
	flushes  int
}

func newFaultStore() *faultStore {
	return &faultStore{
		byHeight: map[uint32][]byte{},
		applied:  map[uint32]int{},
		failAt:   map[uint32]error{},
	}
}

func (s *faultStore) ApplyBlock(b *database.DBBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.failAt[b.Height]; ok {
		return err
	}
	s.byHeight[b.Height] = append([]byte(nil), b.Hash[:]...)
	s.applied[b.Height]++
	return nil
}

func (s *faultStore) FlushBatch(bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushes++
	return nil
}

func (s *faultStore) BatchSize() int { return 0 }

func (s *faultStore) GetChainTip() ([]byte, uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tip uint32
	var hash []byte
	for h, v := range s.byHeight {
		if hash == nil || h > tip {
			tip, hash = h, v
		}
	}
	return hash, tip, nil
}

func (s *faultStore) GetBlockHashByHeight(height uint32) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byHeight[height], nil
}

func (s *faultStore) BlockhashInDB(blockhash []byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.byHeight {
		if bytes.Equal(v, blockhash) {
			return true, nil
		}
	}
	return false, nil
}

// withParallelism sets the indexer's concurrency for one test.
func withParallelism(t *testing.T, requests uint16, workers int) {
	t.Helper()
	oldReq, oldWork := config.MaxParallelRequests, config.MaxParallelTweakComputations
	config.MaxParallelRequests, config.MaxParallelTweakComputations = requests, workers
	t.Cleanup(func() {
		config.MaxParallelRequests, config.MaxParallelTweakComputations = oldReq, oldWork
	})
}

// errDeadlock is returned by returnsWithin when fn did not return in time.
var errDeadlock = errors.New("did not return in time (deadlock)")

// returnsWithin runs fn and fails the test, with a goroutine dump, if it does
// not return within d. The stuck goroutine is left behind on failure; the
// test binary still finishes because the test reports and moves on.
func returnsWithin(t *testing.T, d time.Duration, what string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		t.Errorf("%s %v; goroutines:\n%s", what, errDeadlock, indexerStacks(string(buf)))
		return errDeadlock
	}
}

// indexerStacks keeps only goroutines that run code from this package.
func indexerStacks(all string) string {
	var out []string
	for _, g := range strings.Split(all, "\n\n") {
		if strings.Contains(g, "blindbit-oracle/internal/indexer.") &&
			!strings.Contains(g, "testing.tRunner") &&
			!strings.Contains(g, "(*faultNode).serve") { // the fake node's own handlers
			out = append(out, g)
		}
	}
	return strings.Join(out, "\n\n")
}

// assertNoIndexerGoroutines fails if goroutines started by the sync code are
// still alive shortly after the call returned (a leak, or a stuck sender).
func assertNoIndexerGoroutines(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		left := indexerStacks(string(buf))
		// returnsWithin's own wrapper goroutine is fine once fn has returned
		if left == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("indexer goroutines still running after return:\n%s", left)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
