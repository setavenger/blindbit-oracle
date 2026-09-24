package v2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"slices"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/setavenger/blindbit-lib/proto/pb"
)

func reversed(b []byte) []byte {
	out := slices.Clone(b)
	slices.Reverse(out)
	return out
}

// TestGetFullBlockWireEncoding pins the FullTxItem byte layout that clients
// decode. The only coupling between the oracle and its Rust clients is the
// wire, so a change here is a silent breaking change: this test must only
// ever change together with GRPC.md and every consumer.
//
// Contract: txids and block hashes are in display order (byte-reversed from
// the internal/consensus order), and each 36-byte input outpoint is the
// display-order previous txid followed by the vout as a BIG-endian uint32.
func TestGetFullBlockWireEncoding(t *testing.T) {
	client := newTestClient(t)
	resp, err := client.GetFullBlock(context.Background(), &pb.BlockHeightRequest{BlockHeight: testHeight})
	if err != nil {
		t.Fatalf("GetFullBlock(%d): %v", testHeight, err)
	}

	if got, want := resp.GetBlockIdentifier().GetBlockHash(), reversed(testBlockHash); !bytes.Equal(got, want) {
		t.Errorf("block hash = %x, want display order %x", got, want)
	}
	if got := resp.GetBlockIdentifier().GetBlockHeight(); got != testHeight {
		t.Errorf("block height = %d, want %d", got, testHeight)
	}
	if len(resp.GetIndex()) != 1 {
		t.Fatalf("index has %d items, want 1", len(resp.GetIndex()))
	}
	item := resp.GetIndex()[0]

	if got, want := item.GetTxid(), reversed(testTxid); !bytes.Equal(got, want) {
		t.Errorf("txid = %x, want display order %x", got, want)
	}

	wantInputs := binary.BigEndian.AppendUint32(reversed(testPrevTxid), testPrevVout)
	if got := item.GetInputs(); !bytes.Equal(got, wantInputs) {
		t.Fatalf("inputs = %x, want %x (display-order txid + big-endian vout)", got, wantInputs)
	}
	// Spell the vout bytes out so a switch to little-endian cannot pass by
	// accident: 0x00000102 little-endian would be 02 01 00 00.
	if got := item.GetInputs()[32:36]; !bytes.Equal(got, []byte{0x00, 0x00, 0x01, 0x02}) {
		t.Errorf("input vout bytes = %x, want 00000102 (big-endian)", got)
	}

	if len(item.GetUtxos()) != 1 || item.GetUtxos()[0].GetVout() != 0 || item.GetUtxos()[0].GetAmount() != 50_000 {
		t.Errorf("utxos = %v, want one output at vout 0 worth 50000", item.GetUtxos())
	}
}

// TestUnaryRPCsReturnNotFoundForUnindexedHeights covers the per-height unary
// RPCs: an unindexed height must be NOT_FOUND (as GRPC.md documents), not an
// OK response with an empty hash that looks like an empty block.
func TestUnaryRPCsReturnNotFoundForUnindexedHeights(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	rpcs := map[string]func(uint64) error{
		"GetFullBlock": func(h uint64) error {
			_, err := client.GetFullBlock(ctx, &pb.BlockHeightRequest{BlockHeight: h})
			return err
		},
		"GetSpentOutputsShort": func(h uint64) error {
			_, err := client.GetSpentOutputsShort(ctx, &pb.BlockHeightRequest{BlockHeight: h})
			return err
		},
		"GetBlockHashByHeight": func(h uint64) error {
			_, err := client.GetBlockHashByHeight(ctx, &pb.BlockHeightRequest{BlockHeight: h})
			return err
		},
	}

	unindexed := map[string]uint64{
		"below the first indexed block": testHeight - 1,
		"above the tip":                 testHeight + 1,
		// Would alias to testHeight if it were truncated to uint32.
		"beyond uint32": uint64(math.MaxUint32) + 1 + testHeight,
	}

	for name, rpc := range rpcs {
		if err := rpc(testHeight); err != nil {
			t.Errorf("%s(%d) on an indexed height: %v", name, testHeight, err)
		}
		for why, height := range unindexed {
			if got := status.Code(rpc(height)); got != codes.NotFound {
				t.Errorf("%s(%d) (%s) = %v, want NotFound", name, height, why, got)
			}
		}
	}
}

func TestGetBlockHashAndSpentOutputsForIndexedHeight(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	hash, err := client.GetBlockHashByHeight(ctx, &pb.BlockHeightRequest{BlockHeight: testHeight})
	if err != nil {
		t.Fatalf("GetBlockHashByHeight: %v", err)
	}
	if !bytes.Equal(hash.GetBlockHash(), reversed(testBlockHash)) {
		t.Errorf("block hash = %x, want %x", hash.GetBlockHash(), reversed(testBlockHash))
	}

	spent, err := client.GetSpentOutputsShort(ctx, &pb.BlockHeightRequest{BlockHeight: testHeight})
	if err != nil {
		t.Fatalf("GetSpentOutputsShort: %v", err)
	}
	if want := bytes.Repeat([]byte{0x66}, 8); !bytes.Equal(spent.GetIndex(), want) {
		t.Errorf("spent outputs = %x, want %x", spent.GetIndex(), want)
	}
}

// TestStreamUnindexedHeightBehaviourUnchanged pins that the streaming RPCs
// were deliberately left alone: an unindexed height inside the range still
// yields a message with an empty block hash and no data, and the stream ends
// cleanly. Turning that into a mid-stream NOT_FOUND would abort range scans in
// live clients, so it needs a coordinated change, not a server-only one.
func TestStreamUnindexedHeightBehaviourUnchanged(t *testing.T) {
	client := newTestClient(t)
	stream, err := client.StreamBlockScanDataShort(context.Background(),
		&pb.RangedBlockHeightRequestFiltered{Start: testHeight, End: testHeight + 1})
	if err != nil {
		t.Fatalf("StreamBlockScanDataShort: %v", err)
	}
	var got []*pb.BlockScanDataShortResponse
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		got = append(got, msg)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if !bytes.Equal(got[0].GetBlockIdentifier().GetBlockHash(), reversed(testBlockHash)) || len(got[0].GetCompIndex()) != 1 {
		t.Errorf("indexed height message = %v", got[0])
	}
	if len(got[1].GetBlockIdentifier().GetBlockHash()) != 0 || len(got[1].GetCompIndex()) != 0 {
		t.Errorf("unindexed height message = %v, want empty hash and no data", got[1])
	}
}
