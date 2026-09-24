package indexer

import (
	"bytes"
	"context"
	"testing"

	"github.com/setavenger/blindbit-oracle/internal/config"
)

// TestInitialSyncIndexesConfiguredStartHeight syncs a fresh database against a
// fake node and requires every height from the configured start height to the
// node's tip to be indexed with the node's block hash. Before the fix the
// first height (the configured start height itself) was skipped, so a regtest
// oracle never indexed height 1.
func TestInitialSyncIndexesConfiguredStartHeight(t *testing.T) {
	for _, start := range []uint32{1, 3} {
		chain := newFakeChain(6, "a")
		serveFakeChain(t, chain)
		config.SyncStartHeight = start

		store := newMemStore(t)
		b := NewBuilder(context.Background(), store)
		if err := b.InitialSyncToTip(context.Background()); err != nil {
			t.Fatalf("start %d: initial sync: %v", start, err)
		}

		for h := uint32(0); h <= 6; h++ {
			got, err := store.GetBlockHashByHeight(h)
			if err != nil {
				t.Fatal(err)
			}
			want := chain.hashAt(int(h))
			switch {
			case h < start && got != nil:
				t.Errorf("start %d: height %d below the start height was indexed", start, h)
			case h >= start && !bytes.Equal(got, want[:]):
				t.Errorf("start %d: height %d: indexed hash %x, node hash %x", start, h, got, want[:])
			}
		}
	}
}

// TestInitialSyncStartHeight covers the start-height rule directly, including
// a non-empty database resuming at tip+1 without re-pulling its tip.
func TestInitialSyncStartHeight(t *testing.T) {
	someHash := make([]byte, 32)
	cases := []struct {
		name       string
		tipHash    []byte
		tip, cfg   uint32
		wantHeight int64
	}{
		{"empty db starts at configured height", nil, 0, 1, 1},
		{"empty db, configured genesis", nil, 0, 0, 0},
		{"empty db, mainnet start", nil, 0, 842_579, 842_579},
		{"resume after tip", someHash, 100, 1, 101},
		{"tip below configured start", someHash, 5, 50, 50},
		{"tip at configured start", someHash, 50, 50, 51},
	}
	for _, c := range cases {
		if got := initialSyncStartHeight(c.tipHash, c.tip, c.cfg); got != c.wantHeight {
			t.Errorf("%s: got %d, want %d", c.name, got, c.wantHeight)
		}
	}
}
