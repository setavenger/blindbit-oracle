package dbpebble

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

func newMemStore(t *testing.T) *Store {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func fill32(tag byte) []byte { return bytes.Repeat([]byte{tag}, 32) }

func hashOf(tag byte) *chainhash.Hash {
	h := chainhash.Hash(fill32(tag))
	return &h
}

// spTx is a transaction with a tweak and one taproot output, optionally
// spending a taproot outpoint.
func spTx(tag byte, spends *database.In) *database.Tx {
	txid := fill32(tag)
	var tweak [33]byte
	tweak[0] = 0x02
	copy(tweak[1:], bytes.Repeat([]byte{tag ^ 0x5a}, 32))
	tx := &database.Tx{
		Txid:  txid,
		Tweak: &tweak,
		Outs: []*database.Output{{
			Txid: txid, Vout: 0, Amount: 10_000, Pubkey: fill32(tag ^ 0xa5),
		}},
	}
	if spends != nil {
		in := *spends
		in.SpendTxid = txid
		tx.Ins = []*database.In{&in}
	}
	return tx
}

func block(height uint32, hashTag byte, txs ...*database.Tx) *database.DBBlock {
	// position 0 is the coinbase, which carries no silent-payment data
	return &database.DBBlock{Height: height, Hash: hashOf(hashTag), Txs: append([]*database.Tx{nil}, txs...)}
}

func apply(t *testing.T, s *Store, blocks ...*database.DBBlock) {
	t.Helper()
	for _, b := range blocks {
		if err := s.ApplyBlock(b); err != nil {
			t.Fatalf("apply height %d: %v", b.Height, err)
		}
		if err := s.FlushBatch(true); err != nil {
			t.Fatal(err)
		}
	}
}

// dump returns every key/value in the store as sorted "key=value" strings.
func dump(t *testing.T, s *Store) []string {
	t.Helper()
	it, err := s.DB.NewIter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var out []string
	for ok := it.First(); ok; ok = it.Next() {
		out = append(out, fmt.Sprintf("%x=%x", it.Key(), it.Value()))
	}
	sort.Strings(out)
	return out
}

func ciTxids(t *testing.T, s *Store, height uint32) []string {
	t.Helper()
	items, err := s.FetchComputeIndex(height)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		out = append(out, fmt.Sprintf("%x/%x/%x", it.Txid, it.Tweak, it.OutputsShort))
	}
	sort.Strings(out)
	return out
}

// reorgFixture is the chain used by the reorg tests.
//
//	h1  A1: X (output X:0)
//	h2  A2: R, E                  ->  B2: N
//	h3  A3: S spends X:0          ->  B3: R
//
// After the reorg R is re-confirmed one block later, E is evicted, and the
// spend of X:0 exists only in the orphaned A3.
type reorgFixture struct {
	a1, a2, a3, b2, b3 *database.DBBlock
	x, r, e, s, n      *database.Tx
}

func newReorgFixture() reorgFixture {
	var f reorgFixture
	f.x = spTx(0x10, nil)
	f.r = spTx(0x11, nil)
	f.e = spTx(0x12, nil)
	f.s = spTx(0x13, &database.In{Pubkey: f.x.Outs[0].Pubkey, PrevTxid: f.x.Txid, PrevVout: 0})
	f.n = spTx(0x14, nil)
	f.a1 = block(1, 0xa1, f.x)
	f.a2 = block(2, 0xa2, f.r, f.e)
	f.a3 = block(3, 0xa3, f.s)
	f.b2 = block(2, 0xb2, f.n)
	f.b3 = block(3, 0xb3, f.r)
	return f
}

func TestReorgRemovesDisconnectedBlockData(t *testing.T) {
	orders := map[string]func(f reorgFixture) []*database.DBBlock{
		// the continuous-sync walk-back applies the new branch in ascending order
		"ascending": func(f reorgFixture) []*database.DBBlock { return []*database.DBBlock{f.b2, f.b3} },
		// the pre-fix walk-back applied it tip first; the store must not depend on order
		"descending": func(f reorgFixture) []*database.DBBlock { return []*database.DBBlock{f.b3, f.b2} },
	}
	for name, order := range orders {
		t.Run(name, func(t *testing.T) {
			f := newReorgFixture()
			s := newMemStore(t)
			apply(t, s, f.a1, f.a2, f.a3)

			// sanity: before the reorg X:0 is spent and E is indexed
			if spent, _ := s.spentAtHeightTip(f.x.Txid, 0, 3); !spent {
				t.Fatal("fixture: X:0 should be spent by A3 before the reorg")
			}

			apply(t, s, order(f)...)

			// 1. the whole database equals one that indexed the new chain directly
			fresh := newMemStore(t)
			apply(t, fresh, f.a1, f.b2, f.b3)
			if got, want := dump(t, s), dump(t, fresh); !slices.Equal(got, want) {
				t.Errorf("database after reorg differs from a fresh index of the new chain\nextra: %v\nmissing: %v",
					minus(got, want), minus(want, got))
			}

			// 2. compute index for the reorged heights is exactly the new blocks' data
			for _, h := range []uint32{2, 3} {
				if got, want := ciTxids(t, s, h), ciTxids(t, fresh, h); !slices.Equal(got, want) {
					t.Errorf("compute index at %d: got %v, want %v", h, got, want)
				}
			}
			if got := ciTxids(t, s, 2); len(got) != 1 {
				t.Errorf("compute index at 2 should hold only N, got %v", got)
			}

			// 3. the evicted tx's tweak and outputs are gone
			if _, ok, _ := s.LoadTweak(f.e.Txid); ok {
				t.Error("evicted tx E still has a tweak")
			}
			if outs, _ := s.OutputsForTx(f.e.Txid); len(outs) != 0 {
				t.Errorf("evicted tx E still has %d outputs", len(outs))
			}

			// 4. the spend that existed only in the orphan no longer marks X:0 spent
			if spent, _ := s.spentAtHeightTip(f.x.Txid, 0, 3); spent {
				t.Error("X:0 still reported spent by the orphaned block A3")
			}
			outs, err := s.FetchOutputsCutThroughDustLimit(f.a1.Hash[:], 3, 0)
			if err != nil || len(outs) != 1 {
				t.Errorf("cut-through outputs of A1: got %d (err %v), want X:0", len(outs), err)
			}
			rows, err := s.TweaksForBlockCutThrough(f.a1.Hash[:], 3)
			if err != nil || len(rows) != 1 {
				t.Errorf("cut-through tweaks of A1: got %d (err %v), want X", len(rows), err)
			}

			// 5. orphaned hashes are not on the best chain; the new ones are
			for _, blk := range []*database.DBBlock{f.a2, f.a3} {
				if in, _ := s.BlockhashInDB(blk.Hash[:]); in {
					t.Errorf("orphaned block %x still reported in DB", blk.Hash[:2])
				}
			}
			for _, blk := range []*database.DBBlock{f.a1, f.b2, f.b3} {
				if in, _ := s.BlockhashInDB(blk.Hash[:]); !in {
					t.Errorf("best-chain block %x not reported in DB", blk.Hash[:2])
				}
			}

			// 6. the re-confirmed tx is served at its new height
			if _, ok, _ := s.LoadTweak(f.r.Txid); !ok {
				t.Error("re-confirmed tx R lost its tweak")
			}
			if outs, _ := s.OutputsForTx(f.r.Txid); len(outs) != 1 {
				t.Errorf("re-confirmed tx R has %d outputs, want 1", len(outs))
			}
			tw, _ := s.TweaksForBlockAll(f.b3.Hash[:])
			if len(tw) != 1 || !bytes.Equal(tw[0].Txid[:], f.r.Txid) {
				t.Errorf("tweaks of B3: got %v, want R", tw)
			}

			// 7. no key anywhere still references an orphaned block hash
			for _, kv := range dump(t, s) {
				for _, blk := range []*database.DBBlock{f.a2, f.a3} {
					if bytes.Contains([]byte(kv), []byte(fmt.Sprintf("%x", blk.Hash[:]))) {
						t.Errorf("row still references orphan %x: %s", blk.Hash[:2], kv)
					}
				}
			}
		})
	}
}

// TestReorgSameTxSameHeight covers a tx re-confirmed at the same height: its
// rows are deleted and rewritten in one batch and must survive.
func TestReorgSameTxSameHeight(t *testing.T) {
	f := newReorgFixture()
	s := newMemStore(t)
	apply(t, s, f.a1, f.a2)
	b2 := block(2, 0xb2, f.r, f.n)
	apply(t, s, b2)

	fresh := newMemStore(t)
	apply(t, fresh, f.a1, b2)
	if got, want := dump(t, s), dump(t, fresh); !slices.Equal(got, want) {
		t.Errorf("extra: %v\nmissing: %v", minus(got, want), minus(want, got))
	}
}

// TestReapplySameBlockIsNoop checks re-applying the indexed block leaves the
// database unchanged (the continuous sync may re-apply a block it already has).
func TestReapplySameBlockIsNoop(t *testing.T) {
	f := newReorgFixture()
	s := newMemStore(t)
	apply(t, s, f.a1, f.a2, f.a3)
	before := dump(t, s)
	apply(t, s, f.a3, f.a2)
	if after := dump(t, s); !slices.Equal(before, after) {
		t.Errorf("re-apply changed the database\nextra: %v\nmissing: %v", minus(after, before), minus(before, after))
	}
}

// TestPurgeOrphanedBlocks builds the state a database indexed before reorg
// handling holds after the same reorg (the new blocks were written over the
// old ones and nothing was deleted) and requires PurgeOrphanedBlocks to turn
// it into exactly a fresh index of the new chain.
func TestPurgeOrphanedBlocks(t *testing.T) {
	f := newReorgFixture()
	s := newMemStore(t)
	apply(t, s, f.a1, f.a2, f.a3)

	// the pre-fix write path: overwrite heights 2 and 3 without deleting anything
	for _, blk := range []*database.DBBlock{f.b2, f.b3} {
		b := s.DB.NewBatch()
		if err := attachBlockToBatch(b, blk); err != nil {
			t.Fatal(err)
		}
		if err := b.Commit(pebble.Sync); err != nil {
			t.Fatal(err)
		}
	}
	if in, _ := s.BlockhashInDB(f.a3.Hash[:]); in {
		t.Error("stale orphan A3 reported as in DB before the purge")
	}

	n, err := s.PurgeOrphanedBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("purged %d orphans, want 2", n)
	}

	fresh := newMemStore(t)
	apply(t, fresh, f.a1, f.b2, f.b3)
	if got, want := dump(t, s), dump(t, fresh); !slices.Equal(got, want) {
		t.Errorf("database after purge differs from a fresh index of the new chain\nextra: %v\nmissing: %v",
			minus(got, want), minus(want, got))
	}

	// a second run finds nothing and writes nothing
	before := dump(t, s)
	if n, err := s.PurgeOrphanedBlocks(); err != nil || n != 0 {
		t.Errorf("second purge: %d, %v", n, err)
	}
	if !slices.Equal(before, dump(t, s)) {
		t.Error("second purge changed the database")
	}
}

func minus(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, v := range b {
		in[v] = true
	}
	var out []string
	for _, v := range a {
		if !in[v] {
			out = append(out, v)
		}
	}
	return out
}
