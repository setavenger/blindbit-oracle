package v2

import (
	"testing"

	"google.golang.org/grpc"

	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// filterCall records one FetchComputeIndexFiltered call.
type filterCall struct {
	height     uint32
	tipHeight  uint32
	dustLimit  uint64
	cutThrough bool
}

// stubDB implements database.DB and records only what StreamComputeIndex asks
// of it. Everything else is present to satisfy the interface.
type stubDB struct {
	tip       uint32
	tipCalls  int
	callsMade []filterCall
}

func (d *stubDB) GetChainTip() ([]byte, uint32, error) {
	d.tipCalls++
	return make([]byte, 32), d.tip, nil
}

func (d *stubDB) FetchComputeIndexFiltered(
	height, tipHeight uint32, dustLimit uint64, cutThrough bool,
) ([]*pb.ComputeIndexTxItem, error) {
	d.callsMade = append(d.callsMade, filterCall{
		height: height, tipHeight: tipHeight, dustLimit: dustLimit,
		cutThrough: cutThrough,
	})
	return nil, nil
}

func (d *stubDB) GetBlockHashByHeight(uint32) ([]byte, error) {
	return make([]byte, 32), nil
}

func (d *stubDB) ApplyBlock(*database.DBBlock) error { return nil }
func (d *stubDB) FlushBatch(bool) error              { return nil }
func (d *stubDB) TweaksForBlockAll([]byte) ([]*database.TweakRow, error) {
	return nil, nil
}
func (d *stubDB) TweaksForBlockCutThrough([]byte, uint32) ([]database.TweakRow, error) {
	return nil, nil
}
func (d *stubDB) FetchOutputsAll([]byte, uint32) ([]*database.Output, error) {
	return nil, nil
}
func (d *stubDB) FetchSpentOutputsShort([]byte) ([]byte, error) { return nil, nil }
func (d *stubDB) ChainIterator(bool) (<-chan []byte, error)     { return nil, nil }
func (d *stubDB) FetchComputeIndex(uint32) ([]*pb.ComputeIndexTxItem, error) {
	return nil, nil
}
func (d *stubDB) BlockhashInDB([]byte) (bool, error)         { return false, nil }
func (d *stubDB) BatchSize() int                             { return 0 }
func (d *stubDB) KeyExistsComputeIndex([]byte) (bool, error) { return false, nil }
func (d *stubDB) FetchTxidOutpoints([]byte, []byte) ([][36]byte, error) {
	return nil, nil
}
func (d *stubDB) FetchAllTxidOutpointsForBlock([]byte) (map[[32]byte][][36]byte, error) {
	return nil, nil
}

var _ database.DB = (*stubDB)(nil)

// stubStream swallows the responses.
type stubStream struct {
	grpc.ServerStream
	sent int
}

func (s *stubStream) Send(*pb.ComputeIndexResponse) error {
	s.sent++
	return nil
}

// StreamComputeIndex must hand the request's dustlimit and cut_through to the
// database untouched, and must read the chain tip once for the whole stream.
func TestStreamComputeIndexForwardsFilters(t *testing.T) {
	cases := []struct {
		name      string
		req       *pb.RangedBlockHeightRequestFiltered
		wantTip   uint32 // tip value expected in every call
		wantCalls int
		wantTips  int // how often the chain tip may be read
	}{
		{
			name: "filters set",
			req: &pb.RangedBlockHeightRequestFiltered{
				Start: 10, End: 12, Dustlimit: 1234, CutThrough: true,
			},
			wantTip: 900123, wantCalls: 3, wantTips: 1,
		},
		{
			name: "no filters",
			req: &pb.RangedBlockHeightRequestFiltered{
				Start: 10, End: 12,
			},
			// no cut-through, so no tip is needed and none is read
			wantTip: 0, wantCalls: 3, wantTips: 0,
		},
		{
			name: "dust only",
			req: &pb.RangedBlockHeightRequestFiltered{
				Start: 5, End: 5, Dustlimit: 546,
			},
			wantTip: 0, wantCalls: 1, wantTips: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := &stubDB{tip: 900123}
			stream := &stubStream{}

			if err := NewOracleService(db).StreamComputeIndex(c.req, stream); err != nil {
				t.Fatal(err)
			}

			if len(db.callsMade) != c.wantCalls {
				t.Fatalf("got %d db calls, want %d", len(db.callsMade), c.wantCalls)
			}
			if stream.sent != c.wantCalls {
				t.Errorf("sent %d responses, want %d", stream.sent, c.wantCalls)
			}
			if db.tipCalls != c.wantTips {
				t.Errorf("read the chain tip %d times, want %d", db.tipCalls, c.wantTips)
			}

			for i, call := range db.callsMade {
				wantHeight := uint32(c.req.Start) + uint32(i)
				if call.height != wantHeight {
					t.Errorf("call %d height = %d, want %d", i, call.height, wantHeight)
				}
				if call.dustLimit != c.req.Dustlimit {
					t.Errorf("call %d dustLimit = %d, want %d", i, call.dustLimit, c.req.Dustlimit)
				}
				if call.cutThrough != c.req.CutThrough {
					t.Errorf("call %d cutThrough = %v, want %v", i, call.cutThrough, c.req.CutThrough)
				}
				if call.tipHeight != c.wantTip {
					t.Errorf("call %d tipHeight = %d, want %d", i, call.tipHeight, c.wantTip)
				}
			}
		})
	}
}
