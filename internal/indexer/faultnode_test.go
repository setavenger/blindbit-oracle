package indexer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
	// faultWarmup answers 503 like Bitcoin Core while it loads its block index.
	faultWarmup
	// faultNotFound answers 404 like Core for a block it does not have (e.g.
	// pruned): waiting does not help, so the indexer must not retry it.
	faultNotFound
	// faultUnauthorized answers 401, like a proxy in front of Core that wants
	// credentials: a configuration error.
	faultUnauthorized
	// faultRESTDisabled answers an empty 404, like Core started without rest=1.
	faultRESTDisabled
	// faultTruncate starts a 200 answer and closes the connection mid-body,
	// like a node that stops while it sends a block.
	faultTruncate
)

// faultNode serves the subset of Bitcoin Core's REST interface used by the
// indexer (chaininfo, blockhashbyheight, block, spenttxouts) from a chain of
// coinbase-only blocks, and can inject faults per endpoint and height.
//
// The node can also go offline and come back on the same address, like a
// restarting bitcoind: while it is offline connections are refused.
type faultNode struct {
	t    *testing.T
	addr string // host:port, kept across restarts

	mu      sync.Mutex
	srv     *httptest.Server // nil while offline
	blocks  []*wire.MsgBlock
	heights map[chainhash.Hash]int
	faults  map[string]faultKind // key: "<endpoint>/<height>"
	counts  map[string]int       // remaining answers for a fault; absent = forever
	// nodeFault applies to every request while nodeFaultLeft > 0 (or < 0:
	// forever), like a node that is still starting.
	nodeFault     faultKind
	nodeFaultLeft int
	requests      map[string]int // served requests per endpoint
	onRequest     func(endpoint string, height int)
	stop          chan struct{}
}

func newFaultNode(t *testing.T, tip int) *faultNode {
	t.Helper()
	n := newOfflineFaultNode(t, tip)
	n.goOnline()
	return n
}

// newOfflineFaultNode reserves an address but does not listen on it yet:
// requests are refused until goOnline.
func newOfflineFaultNode(t *testing.T, tip int) *faultNode {
	t.Helper()
	n := &faultNode{
		t:        t,
		heights:  map[chainhash.Hash]int{},
		faults:   map[string]faultKind{},
		counts:   map[string]int{},
		requests: map[string]int{},
		stop:     make(chan struct{}),
	}
	n.extend(tip + 1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("faultnode: reserve address: %v", err)
	}
	n.addr = ln.Addr().String()
	_ = ln.Close()

	oldEndpoint := config.RestEndpoint
	config.RestEndpoint = "http://" + n.addr
	t.Cleanup(func() {
		close(n.stop) // release hanging handlers before closing the server
		n.goOffline()
		config.RestEndpoint = oldEndpoint
	})
	return n
}

// goOnline starts listening on the node's address.
func (n *faultNode) goOnline() {
	n.t.Helper()
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.srv != nil {
		return
	}
	var ln net.Listener
	var err error
	for range 50 { // the port may take a moment to be released
		if ln, err = net.Listen("tcp", n.addr); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		n.t.Fatalf("faultnode: listen on %s: %v", n.addr, err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(n.serve))
	_ = srv.Listener.Close()
	srv.Listener = ln
	srv.Start()
	n.srv = srv
}

// goOffline closes the listener and every open connection: in-flight
// requests fail and new ones are refused, as when bitcoind stops.
func (n *faultNode) goOffline() {
	n.mu.Lock()
	srv := n.srv
	n.srv = nil
	n.mu.Unlock()
	if srv != nil {
		srv.CloseClientConnections()
		srv.Close()
	}
}

// extend appends k blocks to the node's chain, like a node receiving blocks.
func (n *faultNode) extend(k int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	prev := chainhash.Hash{}
	if len(n.blocks) > 0 {
		prev = n.blocks[len(n.blocks)-1].BlockHash()
	}
	for range k {
		h := len(n.blocks)
		blk := faultCoinbaseBlock(prev, h)
		n.blocks = append(n.blocks, blk)
		prev = blk.BlockHash()
		n.heights[prev] = h
	}
}

func (n *faultNode) tip() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.blocks) - 1
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

// inject makes every request for endpoint at height fail with kind.
func (n *faultNode) inject(endpoint string, height int, kind faultKind) {
	n.injectN(endpoint, height, kind, -1)
}

// injectN makes the next count requests for endpoint at height fail with
// kind; count < 0 means every request.
func (n *faultNode) injectN(endpoint string, height int, kind faultKind, count int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := fmt.Sprintf("%s/%d", endpoint, height)
	n.faults[key] = kind
	delete(n.counts, key)
	if count >= 0 {
		n.counts[key] = count
	}
}

// failAll makes the next count requests to any endpoint fail with kind;
// count < 0 means until failAll is called again.
func (n *faultNode) failAll(kind faultKind, count int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nodeFault, n.nodeFaultLeft = kind, count
}

// fault returns the fault for this request and records the request.
func (n *faultNode) fault(endpoint string, height int) faultKind {
	n.mu.Lock()
	n.requests[endpoint]++
	hook := n.onRequest
	kind := faultKind(0)
	if n.nodeFaultLeft != 0 {
		kind = n.nodeFault
		if n.nodeFaultLeft > 0 {
			n.nodeFaultLeft--
		}
	} else {
		key := fmt.Sprintf("%s/%d", endpoint, height)
		kind = n.faults[key]
		if left, ok := n.counts[key]; ok {
			if left == 0 {
				kind = 0
			} else {
				n.counts[key] = left - 1
			}
		}
	}
	n.mu.Unlock()
	if hook != nil {
		hook(endpoint, height)
	}
	return kind
}

func (n *faultNode) requestCount(endpoint string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.requests[endpoint]
}

func (n *faultNode) hashAt(height int) chainhash.Hash {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.blocks[height].BlockHash()
}

func (n *faultNode) blockAt(height int) *wire.MsgBlock {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.blocks[height]
}

// answerFault writes the response for kind and reports whether it did.
func (n *faultNode) answerFault(w http.ResponseWriter, r *http.Request, kind faultKind) bool {
	switch kind {
	case faultDrop:
		hj, ok := w.(http.Hijacker)
		if !ok {
			n.t.Errorf("faultnode: response writer cannot hijack")
			return true
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	case faultHang:
		select {
		case <-r.Context().Done():
		case <-n.stop:
		}
	case faultTruncate:
		hj, ok := w.(http.Hijacker)
		if !ok {
			n.t.Errorf("faultnode: response writer cannot hijack")
			return true
		}
		conn, buf, err := hj.Hijack()
		if err == nil {
			_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n0123456789")
			_ = buf.Flush()
			_ = conn.Close()
		}
	case faultWarmup:
		http.Error(w, "Service temporarily unavailable: Loading block index…", http.StatusServiceUnavailable)
	case faultNotFound:
		http.Error(w, strings.TrimPrefix(r.URL.Path, "/rest/")+" not found", http.StatusNotFound)
	case faultUnauthorized:
		w.WriteHeader(http.StatusUnauthorized)
	case faultRESTDisabled:
		w.WriteHeader(http.StatusNotFound)
	default:
		return false
	}
	return true
}

func (n *faultNode) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/rest/")
	endpoint, arg, _ := strings.Cut(path, "/")

	if path == "chaininfo.json" {
		if n.answerFault(w, r, n.fault("chaininfo", 0)) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain":  "regtest",
			"blocks": n.tip(),
		})
		return
	}

	arg = strings.TrimSuffix(arg, ".bin")
	height := -1
	n.mu.Lock()
	switch endpoint {
	case "blockhashbyheight":
		if h, err := strconv.Atoi(arg); err == nil && h >= 0 && h < len(n.blocks) {
			height = h
		}
	case "block", "spenttxouts":
		if hash, err := chainhash.NewHashFromStr(arg); err == nil {
			if h, ok := n.heights[*hash]; ok {
				height = h
			}
		}
	}
	n.mu.Unlock()
	if height < 0 {
		http.NotFound(w, r)
		return
	}

	if n.answerFault(w, r, n.fault(endpoint, height)) {
		return
	}

	blk := n.blockAt(height)
	switch endpoint {
	case "blockhashbyheight":
		hash := blk.BlockHash()
		_, _ = w.Write(hash[:])
	case "block":
		var buf bytes.Buffer
		if err := blk.Serialize(&buf); err != nil {
			n.t.Errorf("faultnode: serialize block %d: %v", height, err)
			return
		}
		_, _ = w.Write(buf.Bytes())
	case "spenttxouts":
		// one transaction (the coinbase) spending zero outputs
		var buf bytes.Buffer
		_ = wire.WriteVarInt(&buf, 0, uint64(len(blk.Transactions)))
		for range blk.Transactions {
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

func (s *faultStore) indexedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byHeight)
}

func (s *faultStore) appliedAt(height uint32) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applied[height]
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
