package indexer

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type Block struct {
	Height int64
	Hash   *chainhash.Hash
	txs    []*Transaction
}

type Transaction struct {
	ins  []*Vin
	outs []*wire.TxOut
}

type Vin struct {
	txIn    *wire.TxIn
	prevOut *wire.TxOut
}

func (v *Vin) SetTxIn(i *wire.TxIn) {
	v.txIn = i
}

func (v *Vin) SetPrevOut(i *wire.TxOut) {
	v.prevOut = i
}

func (v *Vin) SerialiseKey() ([]byte, error)  { return nil, nil }
func (v *Vin) SerialiseData() ([]byte, error) { return nil, nil }
func (v *Vin) DeSerialiseKey([]byte) error    { return nil }
func (v *Vin) DeSerialiseData([]byte) error   { return nil }
