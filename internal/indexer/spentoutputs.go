package indexer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/btcsuite/btcd/wire"
)

func ParseSpentTxOutsFromBytes(b []byte) ([][]*wire.TxOut, error) {
	r := bytes.NewReader(b)

	return ParseSpentTxOuts(r)
}

func ParseSpentTxOuts(r io.Reader) ([][]*wire.TxOut, error) {
	pver := wire.ProtocolVersion

	nTx, err := wire.ReadVarInt(r, pver) // number of txs incl. coinbase
	if err != nil {
		return nil, fmt.Errorf("read nTx: %w", err)
	}

	res := make([][]*wire.TxOut, nTx)
	for i := range nTx {
		k, err := wire.ReadVarInt(r, pver) // number of spent prevouts for tx i
		if err != nil {
			return nil, fmt.Errorf("tx %d: read k: %w", i, err)
		}

		outs := make([]*wire.TxOut, 0, k)
		for j := range k {
			var val int64
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				return nil, fmt.Errorf("tx %d vin %d: read value: %w", i, j, err)
			}

			spk, err := wire.ReadVarBytes(r, pver, 10000, "scriptPubKey")
			if err != nil {
				return nil, fmt.Errorf("tx %d vin %d: read script: %w", i, j, err)
			}

			outs = append(outs, &wire.TxOut{
				Value:    val,
				PkScript: spk,
			})
		}
		res[i] = outs
	}
	return res, nil
}
