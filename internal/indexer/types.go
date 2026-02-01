package indexer

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type Block struct {
	Height        int64
	Hash          *chainhash.Hash
	PrevBlockHash *chainhash.Hash
	txs           []*Transaction
}

type Transaction struct {
	txid *chainhash.Hash
	ins  []*Vin
	outs []*wire.TxOut
}

// Vin is a container for the original types given by btcsuite which have the relevant data
// we use the original pointers to copy as little as possible
// todo: add convenience methods in-line with future go-bip352 interface spec
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
