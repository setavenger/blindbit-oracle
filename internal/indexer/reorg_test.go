package indexer

import (
	"bytes"
	"context"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// catchUp runs one continuous-sync step the way ContinuousSync does and
// applies whatever it queued for the writer.
func catchUp(t *testing.T, b *Builder, s *dbpebble.Store) {
	t.Helper()
	tipHash, syncTip, err := s.GetChainTip()
	if err != nil {
		t.Fatal(err)
	}
	info, err := GetChainInfo()
	if err != nil {
		t.Fatal(err)
	}
	if !nodeAheadOrForked(info, tipHash, syncTip) {
		t.Fatalf("node at %d (%s) not detected as ahead of or forked from index tip %d",
			info.Blocks, info.BestBlockHash, syncTip)
	}

	b.writerChan = make(chan *database.DBBlock, 1000)
	if err := b.SingleBlockPullAndHandle(context.Background(), uint32(info.Blocks)); err != nil {
		t.Fatal(err)
	}
	close(b.writerChan)
	for blk := range b.writerChan {
		if err := s.ApplyBlock(blk); err != nil {
			t.Fatal(err)
		}
		if err := s.FlushBatch(true); err != nil {
			t.Fatal(err)
		}
	}
}

func indexedHeight(t *testing.T, s *dbpebble.Store, h chainhash.Hash) bool {
	t.Helper()
	in, err := s.BlockhashInDB(h[:])
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// TestContinuousSyncReorg indexes a chain, reorganises the node below the
// index tip, and requires one continuous-sync step to leave the index holding
// exactly the node's chain: every height maps to the node's hash and no
// displaced block hash is still indexed.
func TestContinuousSyncReorg(t *testing.T) {
	cases := []struct {
		name         string
		fork, newTip int
	}{
		{"same height", 3, 6},  // chain length unchanged: only the best hash differs
		{"longer chain", 4, 8}, // disconnect 2, mine 4
		{"tip only", 5, 6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chain := newFakeChain(6, "a")
			serveFakeChain(t, chain)
			config.SyncStartHeight = 1

			store := newMemStore(t)
			b := NewBuilder(context.Background(), store)
			if err := b.InitialSyncToTip(context.Background()); err != nil {
				t.Fatal(err)
			}

			var orphans []chainhash.Hash
			for h := c.fork + 1; h <= 6; h++ {
				orphans = append(orphans, chain.hashAt(h))
			}
			chain.reorg(c.fork, c.newTip, "b")

			catchUp(t, b, store)

			for h := 1; h <= c.newTip; h++ {
				got, err := store.GetBlockHashByHeight(uint32(h))
				if err != nil {
					t.Fatal(err)
				}
				want := chain.hashAt(h)
				if !bytes.Equal(got, want[:]) {
					t.Errorf("height %d: indexed %x, node %x", h, got, want[:])
				}
				if !indexedHeight(t, store, want) {
					t.Errorf("height %d: node block not reported in DB", h)
				}
			}
			for _, o := range orphans {
				if indexedHeight(t, store, o) {
					t.Errorf("orphaned block %s still reported in DB", o)
				}
				if _, closer, err := store.DB.Get(dbpebble.KeyCIBlock(o[:])); err == nil {
					closer.Close()
					t.Errorf("orphaned block %s still has a blockhash->height row", o)
				}
			}
			_, tip, _ := store.GetChainTip()
			if int(tip) != c.newTip {
				t.Errorf("index tip %d, node tip %d", tip, c.newTip)
			}
		})
	}
}
