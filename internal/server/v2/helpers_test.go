package v2

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// Fixture: one indexed block at testHeight holding one taproot-eligible
// transaction that spends a single outpoint whose vout is testPrevVout.
// testPrevVout is chosen so that its big- and little-endian encodings differ
// (0x00000102), which is what lets the wire-format test tell them apart.
const (
	testHeight   = 100
	testPrevVout = 0x00000102
)

// fill returns 32 bytes counting up from start, so internal (as stored) and
// display (reversed) order are distinguishable.
func fill(start byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = start + byte(i)
	}
	return b
}

var (
	testBlockHash = fill(0x10) // internal byte order, as stored
	testTxid      = fill(0x40) // internal byte order, as stored
	testPrevTxid  = fill(0x80) // internal byte order, as stored
)

func newTestStore(t *testing.T) *dbpebble.Store {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	if err != nil {
		t.Fatalf("open in-memory pebble: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := dbpebble.NewStore(db)
	var hash chainhash.Hash
	copy(hash[:], testBlockHash)
	tweak := [33]byte{0x02, 0xaa}
	pubkey := bytes.Repeat([]byte{0x55}, 32)
	prevPubkey := bytes.Repeat([]byte{0x66}, 32)
	block := &database.DBBlock{
		Height: testHeight,
		Hash:   &hash,
		Txs: []*database.Tx{{
			Txid:  testTxid,
			Tweak: &tweak,
			Outs:  []*database.Output{{Txid: testTxid, Vout: 0, Amount: 50_000, Pubkey: pubkey}},
			Ins: []*database.In{{
				SpendTxid: testTxid,
				PrevTxid:  testPrevTxid,
				PrevVout:  testPrevVout,
				Pubkey:    prevPubkey,
			}},
		}},
	}
	if err := store.ApplyBlock(block); err != nil {
		t.Fatalf("apply block: %v", err)
	}
	if err := store.FlushBatch(true); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return store
}

func newTestClient(t *testing.T) pb.OracleServiceClient {
	t.Helper()
	store := newTestStore(t)

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
