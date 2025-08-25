package dbpebble

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// Best-chain map to test membership quickly.

type ActiveHeight func(blockHash []byte) (height uint32, ok bool)

// GetChainTip returns the Blockhash and height of the highest block
func (s *Store) GetChainTip() ([]byte, uint32, error) {
	lb, ub := BoundsCIHeight()
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, 0, err
	}
	defer it.Close()
	if !it.Last() {
		// edge case empty db we are at 0 height
		return nil, 0, nil
	}
	heightBytes := it.Key()
	blockhash := it.Value()

	if len(blockhash) != 32 {
		return nil, 0, fmt.Errorf("bad blockhash %x", blockhash)
	}

	height := binary.BigEndian.Uint32(heightBytes[1:])

	return blockhash, height, nil
}

func (s *Store) GetBlockHashByHeight(height uint32) ([]byte, error) {
	key := KeyCIHeight(height)
	blockhash, closer, err := s.DB.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, fmt.Errorf("height not found in chain index %d", height)
		}
		return nil, err
	}
	defer closer.Close()
	return blockhash, err
}

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

func (s *Store) OutputsForTx(txid []byte) ([]*database.Output, error) {
	lb, ub := BoundsOut(txid)
	it, err := s.DB.NewIter(&pebble.IterOptions{LowerBound: lb, UpperBound: ub})
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var outs []*database.Output
	for ok := it.First(); ok; ok = it.Next() {
		// parse vout from last 4 bytes of key
		k := it.Key()
		vout := binary.BigEndian.Uint32(k[len(k)-SizeVout:])

		amt, pk, err := ParseOutValue(it.Value())
		if err != nil {
			return nil, err
		}
		outs = append(outs, &database.Output{Txid: txid, Vout: vout, Amount: amt, Pubkey: pk})
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

// IsSpentAt Is outpoint spent on best chain at height H?
func (s *Store) IsSpentAt(
	prevTxid []byte, prevVout, H uint32, active ActiveHeight,
) (bool, error) {
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
) ([]*database.TweakRow, error) {
	txids, err := s.BlockTxids(blockHash)
	if err != nil {
		return nil, err
	}

	var out []*database.TweakRow
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
			row := &database.TweakRow{Txid: make([]byte, SizeTxid), Tweak: make([]byte, SizeTweak)}
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
	// todo: should we drop the H thing. Just check if in or not

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

func (s *Store) FetchOutputsAll(
	blockhash []byte, tipHeight uint32,
) ([]*database.Output, error) {
	return s.fetchOutputs(blockhash)
}

func (s *Store) FetchOutputsCutThroughDustLimit(
	blockhash []byte, tipHeight uint32, dustLimit uint64,
) ([]*database.Output, error) {
	timeStart := time.Now()
	defer func() {
		logging.L.Trace().
			Dur("duration", time.Since(timeStart)).
			Uint64("dust_limit", dustLimit).
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("fetching_outputs_filtered_timing")
	}()

	outputs, err := s.fetchOutputs(blockhash)
	if err != nil {
		return nil, err
	}

	filteredOuts := make([]*database.Output, len(outputs))
	idxCounter := 0
	for i := range outputs {
		o := outputs[i]
		if o.Amount < dustLimit {
			continue
		}
		var spent bool
		spent, err = s.spentAtHeightTip(o.Txid, o.Vout, tipHeight)
		if err != nil {
			return nil, err
		}
		if !spent {
			continue
		}

		// passed all filters
		filteredOuts[idxCounter] = o
		idxCounter++
	}

	return filteredOuts[:idxCounter], err
}

func (s *Store) fetchOutputs(
	blockhash []byte,
) ([]*database.Output, error) {
	// timing block on trace level
	timeStart := time.Now()
	defer func() {
		logging.L.Trace().
			Dur("duration", time.Since(timeStart)).
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("fetching_outputs_timing")
	}()

	txids, err := s.BlockTxids(blockhash)
	if err != nil {
		return nil, err
	}

	var out []*database.Output
	out = make([]*database.Output, 0, 100_000)
	for _, txid := range txids {
		outs, err := s.OutputsForTx(txid)
		if err != nil {
			return nil, err
		}

		out = append(out, outs...)
	}

	return out, nil
}

func (s *Store) TweaksForBlockAll(blockhash []byte) ([]database.TweakRow, error) {
	timeStart := time.Now()
	defer func() {
		logging.L.Trace().
			Dur("duration", time.Since(timeStart)).
			Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
			Msg("fetching_tweaks_timing")
	}()
	txids, err := s.BlockTxids(blockhash)
	if err != nil {
		return nil, err
	}

	var out []database.TweakRow
	for _, txid := range txids {
		tweak, ok, err := s.LoadTweak(txid)
		if err != nil {
			return nil, err
		}
		if !ok {
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

// TweaksForBlockCutThrough Account for cut-through
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
		if !ok {
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

func (s *Store) TweaksForBlockCutThroughDustLimit(
	blockHash []byte, tipHeight uint32, dustLimit uint64,
) ([]database.TweakRow, error) {
	// todo: spent at height
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
		if !ok {
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
