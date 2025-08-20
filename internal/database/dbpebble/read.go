package dbpebble

import (
	"encoding/binary"
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// Best-chain map to test membership quickly.
type ActiveHeight func(blockHash []byte) (height uint32, ok bool)

func (s *Store) BlockTxids(blockHash []byte) ([][]byte, error) {
	lb, ub := BoundsBlockTx(blockHash)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var out [][]byte
	for ok := it.First(); ok; ok = it.Next() {
		val := make([]byte, SizeTxid)
		copy(val, it.Value())
		out = append(out, val)
	}
	return out, nil
}

type Output struct {
	Vout   uint32
	Amount uint64
	Pubkey []byte
}

func (s *Store) OutputsForTx(txid []byte) ([]Output, error) {
	lb, ub := BoundsOut(txid)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var outs []Output
	for ok := it.First(); ok; ok = it.Next() {
		// parse vout from last 4 bytes of key
		k := it.Key()
		vout := binary.BigEndian.Uint32(k[len(k)-SizeVout:])

		amt, pk, err := ParseOutValue(it.Value())
		if err != nil {
			return nil, err
		}
		outs = append(outs, Output{Vout: vout, Amount: amt, Pubkey: pk})
	}
	return outs, nil
}

func (s *Store) LoadTweak(txid []byte) ([]byte, bool, error) {
	val, closer, err := s.DB.Get(KeyTx(txid))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer closer.Close()
	out := make([]byte, len(val))
	copy(out, val)
	return out, true, nil
}

// Is outpoint spent on best chain at height H?
func (s *Store) IsSpentAt(prevTxid []byte, prevVout, H uint32, active ActiveHeight) (bool, error) {
	lb, ub := BoundsSpend(prevTxid, prevVout)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return false, err
	}
	defer it.Close()

	for ok := it.First(); ok; ok = it.Next() {
		k := it.Key()
		// blockHash is the last 32 bytes of key
		blk := k[len(k)-SizeHash:]
		if h, ok := active(blk); ok && h <= H {
			return true, nil
		}
	}
	return false, nil
}

// Full query: tweaks for block with cut-through + dust >= X at height H

func (s *Store) TweaksForBlock(
	blockHash []byte,
	H uint32,
	dust uint64,
	active ActiveHeight,
) ([]database.TweakRow, error) {
	txids, err := s.BlockTxids(blockHash)
	if err != nil {
		return nil, err
	}

	var out []database.TweakRow
	for _, txid := range txids {
		tweak, ok, err := s.LoadTweak(txid)
		if err != nil || !ok {
			continue
		} // skip non-SP

		outs, err := s.OutputsForTx(txid)
		if err != nil {
			return nil, err
		}

		var hasUnspent bool
		var maxUnspent uint64
		for _, o := range outs {
			spent, err := s.IsSpentAt(txid, o.Vout, H, active)
			if err != nil {
				return nil, err
			}
			if !spent {
				hasUnspent = true
				if o.Amount > maxUnspent {
					maxUnspent = o.Amount
				}
			}
		}
		if hasUnspent && (dust == 0 || maxUnspent >= dust) {
			row := database.TweakRow{Txid: make([]byte, SizeTxid), Tweak: make([]byte, SizeTweak)}
			copy(row.Txid, txid)
			copy(row.Tweak, tweak)
			out = append(out, row)
		}
	}
	return out, nil

}

// --- new internal helpers ---------------------------------------------------

// heightIfOnBestChain returns (height,true) if blockHash is on best chain; otherwise (0,false).
func (s *Store) heightIfOnBestChain(blockHash []byte) (uint32, bool, error) {
	val, closer, err := s.DB.Get(KeyCIBlock(blockHash)) // ci:b:<blockHash> -> [4]heightBE
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	defer closer.Close()
	if len(val) != SizeHeight {
		return 0, false, errors.New("bad ci:b value length")
	}
	h := binary.BigEndian.Uint32(val[:SizeHeight])
	return h, true, nil
}

// spentAtHeightTip: is (prevTxid,prevVout) spent on best chain at or before H?
func (s *Store) spentAtHeightTip(prevTxid []byte, prevVout, H uint32) (bool, error) {
	lb, ub := BoundsSpend(prevTxid, prevVout) // sp:<txid>:<vout>:<blockHash>
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return false, err
	}
	defer it.Close()

	for ok := it.First(); ok; ok = it.Next() {
		k := it.Key()
		blk := k[len(k)-SizeHash:] // last 32 bytes
		if h, ok, err := s.heightIfOnBestChain(blk); err != nil {
			return false, err
		} else if ok && h <= H {
			return true, nil
		}
	}
	return false, nil
}

// --- API you asked for ------------------------------------------------------

func (s *Store) TweaksForBlockAll(blockHash []byte) ([]database.TweakRow, error) {
	txids, err := s.BlockTxids(blockHash)
	if err != nil {
		return nil, err
	}

	var out []database.TweakRow
	for _, txid := range txids {
		tweak, ok, err := s.LoadTweak(txid)
		if err != nil {
			return nil, err
		}
		if !ok { // not SP-eligible (no tweak stored)
			continue
		}
		row := database.TweakRow{
			Txid:  make([]byte, SizeTxid),
			Tweak: make([]byte, SizeTweak),
		}
		copy(row.Txid, txid)
		copy(row.Tweak, tweak)
		out = append(out, row)
	}
	return out, nil
}

//  2. Cut-through: exclude txs whose every tracked output is already spent
//     at or before tipHeight on the best chain.
func (s *Store) TweaksForBlockCutThrough(
	blockHash []byte, tipHeight uint32,
) ([]database.TweakRow, error) {
	txids, err := s.BlockTxids(blockHash)
	if err != nil {
		return nil, err
	}

	var out []database.TweakRow
	for _, txid := range txids {
		tweak, ok, err := s.LoadTweak(txid)
		if err != nil {
			return nil, err
		}
		if !ok { // not SP-eligible
			continue
		}

		outs, err := s.OutputsForTx(txid)
		if err != nil {
			return nil, err
		}

		// keep tweak if ANY tracked output is unspent at tip
		keep := false
		for _, o := range outs {
			spent, err := s.spentAtHeightTip(txid, o.Vout, tipHeight)
			if err != nil {
				return nil, err
			}
			if !spent {
				keep = true
				break
			}
		}
		if keep {
			row := database.TweakRow{
				Txid:  make([]byte, SizeTxid),
				Tweak: make([]byte, SizeTweak),
			}
			copy(row.Txid, txid)
			copy(row.Tweak, tweak)
			out = append(out, row)
		}
	}
	return out, nil
}
