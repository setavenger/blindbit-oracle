package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// fakeChain serves the subset of Bitcoin Core's REST interface the indexer
// uses (chaininfo, blockhashbyheight, block, spenttxouts) from an in-memory
// chain of coinbase-only blocks. blocks[h] is the block at height h.
type fakeChain struct {
	mu     sync.Mutex
	blocks []*wire.MsgBlock
}

// newFakeChain builds a chain from genesis to tip; tag is mixed into every
// coinbase so two chains built with different tags have different hashes.
func newFakeChain(tip int, tag string) *fakeChain {
	c := &fakeChain{}
	c.blocks = append(c.blocks, coinbaseBlock(chainhash.Hash{}, 0, tag))
	c.extend(tip, tag)
	return c
}

func coinbaseBlock(prev chainhash.Hash, height int, tag string) *wire.MsgBlock {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte(fmt.Sprintf("%d/%s", height, tag)),
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 50_0000_0000, PkScript: []byte{0x51}})

	blk := wire.NewMsgBlock(&wire.BlockHeader{
		Version:   4,
		PrevBlock: prev,
		Timestamp: time.Unix(1_700_000_000+int64(height), 0),
	})
	_ = blk.AddTransaction(tx)
	blk.Header.MerkleRoot = tx.TxHash()
	return blk
}

// extend mines blocks on top of the current tip until the tip is at height tip.
func (c *fakeChain) extend(tip int, tag string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for h := len(c.blocks); h <= tip; h++ {
		c.blocks = append(c.blocks, coinbaseBlock(c.blocks[h-1].BlockHash(), h, tag))
	}
}

// reorg disconnects every block above forkHeight and mines a competing branch
// up to newTip.
func (c *fakeChain) reorg(forkHeight, newTip int, tag string) {
	c.mu.Lock()
	c.blocks = c.blocks[:forkHeight+1]
	c.mu.Unlock()
	c.extend(newTip, tag)
}

func (c *fakeChain) hashAt(h int) chainhash.Hash {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.blocks[h].BlockHash()
}

func (c *fakeChain) byHash(s string) *wire.MsgBlock {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, b := range c.blocks {
		if b.BlockHash().String() == s {
			return b
		}
	}
	return nil
}

func (c *fakeChain) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/rest/chaininfo.json":
		c.mu.Lock()
		tip := len(c.blocks) - 1
		best := c.blocks[tip].BlockHash().String()
		c.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain": "regtest", "blocks": tip, "headers": tip, "bestblockhash": best,
		})
	case strings.HasPrefix(p, "/rest/blockhashbyheight/"):
		h, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(p, "/rest/blockhashbyheight/"), ".bin"))
		c.mu.Lock()
		ok := err == nil && h >= 0 && h < len(c.blocks)
		c.mu.Unlock()
		if !ok {
			http.Error(w, "height out of range", http.StatusNotFound)
			return
		}
		hash := c.hashAt(h)
		_, _ = w.Write(hash[:])
	case strings.HasPrefix(p, "/rest/block/"):
		b := c.byHash(strings.TrimSuffix(strings.TrimPrefix(p, "/rest/block/"), ".bin"))
		if b == nil {
			http.Error(w, "unknown block", http.StatusNotFound)
			return
		}
		var buf bytes.Buffer
		_ = b.Serialize(&buf)
		_, _ = w.Write(buf.Bytes())
	case strings.HasPrefix(p, "/rest/spenttxouts/"):
		b := c.byHash(strings.TrimSuffix(strings.TrimPrefix(p, "/rest/spenttxouts/"), ".bin"))
		if b == nil {
			http.Error(w, "unknown block", http.StatusNotFound)
			return
		}
		// coinbase-only blocks: one tx, zero spent prevouts
		var buf bytes.Buffer
		_ = wire.WriteVarInt(&buf, 0, uint64(len(b.Transactions)))
		for range b.Transactions {
			_ = wire.WriteVarInt(&buf, 0, 0)
		}
		_, _ = w.Write(buf.Bytes())
	default:
		http.NotFound(w, r)
	}
}

// serveFakeChain points the indexer's REST client at c for the test's duration.
func serveFakeChain(t *testing.T, c *fakeChain) {
	t.Helper()
	srv := httptest.NewServer(c)
	t.Cleanup(srv.Close)

	prevEndpoint, prevStart := config.RestEndpoint, config.SyncStartHeight
	config.RestEndpoint = srv.URL
	t.Cleanup(func() {
		config.RestEndpoint, config.SyncStartHeight = prevEndpoint, prevStart
	})
}

// newMemStore opens a pebble store on an in-memory filesystem.
func newMemStore(t *testing.T) *dbpebble.Store {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	s := dbpebble.NewStore(db)
	t.Cleanup(func() { _ = s.Close() })
	return s
}
