package indexer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPullBlockDataTransportErrors(t *testing.T) {
	cases := []struct {
		name   string
		faults []string
	}{
		{"spenttxouts fails", []string{"spenttxouts"}},
		{"block fails", []string{"block"}},
		{"both fail", []string{"spenttxouts", "block"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newFaultNode(t, 5)
			for _, ep := range tc.faults {
				node.inject(ep, 3, faultDrop)
			}
			hash := node.hashAt(3)

			var blk *Block
			err := returnsWithin(t, syncTimeout, "PullBlockData", func() error {
				var err error
				blk, err = PullBlockData(context.Background(), &hash)
				return err
			})
			if errors.Is(err, errDeadlock) {
				return
			}
			if err == nil || blk != nil {
				t.Fatalf("PullBlockData = (%v, %v), want (nil, error)", blk, err)
			}
			assertNoIndexerGoroutines(t)
		})
	}
}

func TestPullBlockDataOK(t *testing.T) {
	node := newFaultNode(t, 5)
	hash := node.hashAt(4)
	blk, err := PullBlockData(context.Background(), &hash)
	if err != nil {
		t.Fatalf("PullBlockData: %v", err)
	}
	if !blk.Hash.IsEqual(&hash) {
		t.Fatalf("hash = %s, want %s", blk.Hash, hash)
	}
}

func TestPullBlockDataCancel(t *testing.T) {
	node := newFaultNode(t, 5)
	node.inject("block", 2, faultHang)
	hash := node.hashAt(2)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := returnsWithin(t, syncTimeout, "PullBlockData", func() error {
		_, err := PullBlockData(ctx, &hash)
		return err
	})
	if errors.Is(err, errDeadlock) {
		return
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("PullBlockData error = %v, want context.DeadlineExceeded", err)
	}
}
