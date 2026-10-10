package indexer

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// These tests only use SyncBlocks/ContinuousSync, whose signatures did not
// change, so they also run against the code before the fix and reproduce the
// original deadlock there.

// syncTimeout bounds every sync call. Healthy runs finish in milliseconds;
// the HTTP client's own timeout (100 s) is far above this, so a pass proves
// that errors and cancellation propagate without waiting for it.
const syncTimeout = 10 * time.Second

func TestSyncBlocksHappyPath(t *testing.T) {
	withParallelism(t, 2, 2)
	node := newFaultNode(t, 60)
	store := newFaultStore()
	b := NewBuilder(context.Background(), store)

	err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
		return b.SyncBlocks(context.Background(), 1, 60)
	})
	if err != nil {
		t.Fatalf("SyncBlocks: %v", err)
	}
	for h := 1; h <= 60; h++ {
		want := node.hashAt(h)
		got, _ := store.GetBlockHashByHeight(uint32(h))
		if !bytes.Equal(got, want[:]) {
			t.Errorf("height %d: stored %x, node %x", h, got, want[:])
		}
		if n := store.applied[uint32(h)]; n != 1 {
			t.Errorf("height %d applied %d times, want 1", h, n)
		}
	}
	if store.flushes != 1 {
		t.Errorf("FlushBatch called %d times, want 1", store.flushes)
	}
	assertNoIndexerGoroutines(t)
}

// TestSyncBlocksPullTransportError is the regression test for the production
// incident: one failed REST request (there: a 100 s client timeout) left the
// indexer blocked forever with the process still running.
func TestSyncBlocksPullTransportError(t *testing.T) {
	for _, endpoint := range []string{"spenttxouts", "block", "blockhashbyheight"} {
		t.Run(endpoint, func(t *testing.T) {
			withParallelism(t, 1, 2)
			node := newFaultNode(t, 30)
			node.inject(endpoint, 15, faultDrop)
			b := NewBuilder(context.Background(), newFaultStore())

			err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
				return b.SyncBlocks(context.Background(), 1, 30)
			})
			if errors.Is(err, errDeadlock) {
				return // already reported with a goroutine dump
			}
			if err == nil {
				t.Fatalf("SyncBlocks returned nil, want the transport error")
			}
			assertNoIndexerGoroutines(t)
		})
	}
}

// Several pulls failing at the same time must not block each other while
// reporting their errors.
func TestSyncBlocksConcurrentPullErrors(t *testing.T) {
	withParallelism(t, 4, 2)
	node := newFaultNode(t, 40)
	for _, h := range []int{10, 11, 12, 13} {
		node.inject("spenttxouts", h, faultDrop)
		node.inject("block", h, faultDrop)
	}
	b := NewBuilder(context.Background(), newFaultStore())

	err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
		return b.SyncBlocks(context.Background(), 1, 40)
	})
	if errors.Is(err, errDeadlock) {
		return
	}
	if err == nil {
		t.Fatalf("SyncBlocks returned nil, want an error")
	}
	assertNoIndexerGoroutines(t)
}

// If the writer fails, workers that are waiting to hand it blocks must not
// block forever (the chain is longer than both channel buffers).
func TestSyncBlocksWriterError(t *testing.T) {
	withParallelism(t, 1, 2)
	newFaultNode(t, 150)
	store := newFaultStore()
	errWrite := errors.New("injected write failure")
	store.failAt[5] = errWrite
	b := NewBuilder(context.Background(), store)

	err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
		return b.SyncBlocks(context.Background(), 1, 150)
	})
	if errors.Is(err, errDeadlock) {
		return
	}
	if !errors.Is(err, errWrite) {
		t.Fatalf("SyncBlocks error = %v, want %v", err, errWrite)
	}
	assertNoIndexerGoroutines(t)
}

// Cancelling the caller's context must stop the sync promptly, including a
// request that the node never answers.
func TestSyncBlocksCancelWithHangingRequest(t *testing.T) {
	withParallelism(t, 2, 2)
	node := newFaultNode(t, 30)
	node.inject("spenttxouts", 8, faultHang)
	b := NewBuilder(context.Background(), newFaultStore())

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)

	start := time.Now()
	err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
		return b.SyncBlocks(ctx, 1, 30)
	})
	if errors.Is(err, errDeadlock) {
		return
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SyncBlocks error = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("SyncBlocks took %v to honour cancellation", d)
	}
	assertNoIndexerGoroutines(t)
}

// ContinuousSync must return the writer's error rather than block or mask it.
func TestContinuousSyncWriterError(t *testing.T) {
	withParallelism(t, 1, 1)
	newFaultNode(t, 3)
	store := newFaultStore()
	errWrite := errors.New("injected write failure")
	store.failAt[3] = errWrite
	b := NewBuilder(context.Background(), store)

	// ContinuousSync polls every 3 s.
	err := returnsWithin(t, syncTimeout, "ContinuousSync", func() error {
		return b.ContinuousSync(context.Background())
	})
	if errors.Is(err, errDeadlock) {
		return
	}
	if !errors.Is(err, errWrite) {
		t.Fatalf("ContinuousSync error = %v, want %v", err, errWrite)
	}
	assertNoIndexerGoroutines(t)
}
