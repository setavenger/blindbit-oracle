package v2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// Stream fixture: blocks indexed at streamLow and streamLow+2, with a gap at
// streamLow+1. streamLow is the lowest indexed height and streamLow+2 the tip.
const streamLow uint64 = 500

// streamHash returns a distinct 32-byte block hash (internal byte order) per
// height, with no two bytes equal so the reversed form is distinguishable.
func streamHash(height uint64) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(height) + byte(i)
	}
	return b
}

func streamReversed(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}

func newStreamTestClient(t *testing.T) pb.OracleServiceClient {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatalf("open in-memory pebble: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := dbpebble.NewStore(db)

	for _, height := range []uint64{streamLow, streamLow + 2} {
		var hash chainhash.Hash
		copy(hash[:], streamHash(height))
		txid := bytes.Repeat([]byte{byte(height) ^ 0x40}, 32)
		tweak := [33]byte{0x02, byte(height)}
		block := &database.DBBlock{
			Height: uint32(height),
			Hash:   &hash,
			Txs: []*database.Tx{{
				Txid:  txid,
				Tweak: &tweak,
				Outs: []*database.Output{{
					Txid: txid, Vout: 0, Amount: 50_000,
					Pubkey: bytes.Repeat([]byte{0x55}, 32),
				}},
			}},
		}
		if err := store.ApplyBlock(block); err != nil {
			t.Fatalf("apply block %d: %v", height, err)
		}
	}
	if err := store.FlushBatch(true); err != nil {
		t.Fatalf("flush: %v", err)
	}

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	pb.RegisterOracleServiceServer(srv, NewOracleService(store))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewOracleServiceClient(conn)
}

// streamMsg is the part of a stream message the tests check.
type streamMsg struct {
	height uint64
	hash   []byte
}

// streamOpener runs one of the two streaming RPCs to completion and returns
// the messages received plus the terminal error (nil on a clean io.EOF).
type streamOpener func(
	ctx context.Context, c pb.OracleServiceClient, start, end uint64,
) ([]streamMsg, error)

func drainScanDataShort(
	ctx context.Context, c pb.OracleServiceClient, start, end uint64,
) ([]streamMsg, error) {
	s, err := c.StreamBlockScanDataShort(ctx,
		&pb.RangedBlockHeightRequestFiltered{Start: start, End: end})
	if err != nil {
		return nil, err
	}
	var msgs []streamMsg
	for {
		m, err := s.Recv()
		if errors.Is(err, io.EOF) {
			return msgs, nil
		}
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, streamMsg{
			height: m.GetBlockIdentifier().GetBlockHeight(),
			hash:   m.GetBlockIdentifier().GetBlockHash(),
		})
	}
}

func drainComputeIndex(
	ctx context.Context, c pb.OracleServiceClient, start, end uint64,
) ([]streamMsg, error) {
	s, err := c.StreamComputeIndex(ctx,
		&pb.RangedBlockHeightRequestFiltered{Start: start, End: end})
	if err != nil {
		return nil, err
	}
	var msgs []streamMsg
	for {
		m, err := s.Recv()
		if errors.Is(err, io.EOF) {
			return msgs, nil
		}
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, streamMsg{
			height: m.GetBlockIdentifier().GetBlockHeight(),
			hash:   m.GetBlockIdentifier().GetBlockHash(),
		})
	}
}

func TestStreamsUnindexedHeightNotFound(t *testing.T) {
	streams := map[string]streamOpener{
		"StreamBlockScanDataShort": drainScanDataShort,
		"StreamComputeIndex":       drainComputeIndex,
	}
	cases := []struct {
		name       string
		start, end uint64
		wantMsgs   []uint64 // heights expected before the stream ends
		notFoundAt uint64   // 0 means the stream must end cleanly
	}{
		{"indexed single height", streamLow, streamLow, []uint64{streamLow}, 0},
		{"indexed tip", streamLow + 2, streamLow + 2, []uint64{streamLow + 2}, 0},
		{"gap inside range", streamLow, streamLow + 2, []uint64{streamLow}, streamLow + 1},
		{"start below first indexed block", streamLow - 1, streamLow, nil, streamLow - 1},
		{"above tip", streamLow + 3, streamLow + 3, nil, streamLow + 3},
		{"range crossing tip", streamLow + 2, streamLow + 3, []uint64{streamLow + 2}, streamLow + 3},
		{"beyond uint32 aliasing an indexed height", 1<<32 + streamLow, 1<<32 + streamLow, nil, 1<<32 + streamLow},
	}

	for streamName, open := range streams {
		for _, tc := range cases {
			t.Run(streamName+"/"+tc.name, func(t *testing.T) {
				client := newStreamTestClient(t)
				msgs, err := open(context.Background(), client, tc.start, tc.end)

				if len(msgs) != len(tc.wantMsgs) {
					t.Fatalf("got %d messages %v, want heights %v (err=%v)",
						len(msgs), msgs, tc.wantMsgs, err)
				}
				for i, want := range tc.wantMsgs {
					if msgs[i].height != want {
						t.Errorf("message %d: height %d, want %d", i, msgs[i].height, want)
					}
					if !bytes.Equal(msgs[i].hash, streamReversed(streamHash(want))) {
						t.Errorf("message %d: block hash %x, want %x",
							i, msgs[i].hash, streamReversed(streamHash(want)))
					}
				}

				if tc.notFoundAt == 0 {
					if err != nil {
						t.Fatalf("stream ended with %v, want clean end", err)
					}
					return
				}
				if status.Code(err) != codes.NotFound {
					t.Fatalf("stream ended with %v (code %s), want NotFound", err, status.Code(err))
				}
				if msg := status.Convert(err).Message(); !strings.Contains(msg, fmt.Sprint(tc.notFoundAt)) {
					t.Errorf("NotFound message %q does not name height %d", msg, tc.notFoundAt)
				}
			})
		}
	}
}
