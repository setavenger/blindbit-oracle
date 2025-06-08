package bitcoinkernel

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
)

// Ensure KernelBlock implements indexer.BlockData
var _ indexer.BlockData = (*KernelBlock)(nil)

type KernelBlock struct {
	btcutil.Block
	blockHeight uint32
}

func NewKernelBlock(blockData []byte, height uint32) *KernelBlock {
	block, err := btcutil.NewBlockFromBytes(blockData)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create block from bytes")
		return nil
	}
	return &KernelBlock{
		Block:       *block,
		blockHeight: height,
	}
}

func (b *KernelBlock) GetBlockHash() [32]byte {
	hash := b.Hash()
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

func (b *KernelBlock) GetBlockHashSlice() []byte {
	hash := b.Hash()
	return hash[:]
}

func (b *KernelBlock) GetBlockHeight() uint32 {
	return b.blockHeight
}

func (b *KernelBlock) GetTransactions() []indexer.Transaction {
	txs := make([]indexer.Transaction, len(b.Transactions()))
	for i, tx := range b.Transactions() {
		msgTx := tx.MsgTx()
		kernelTx := &KernelBitcoinTransaction{
			tx: msgTx,
		}

		kernelTx.outputs = make([]*KernelBitcoinTxOut, len(msgTx.TxOut))
		for j, out := range msgTx.TxOut {
			kernelTx.outputs[j] = &KernelBitcoinTxOut{
				txOut: out,
				outpoint: &wire.OutPoint{
					Hash:  msgTx.TxHash(),
					Index: uint32(j),
				},
				parentTx: kernelTx,
			}
		}

		kernelTx.inputs = make([]*KernelBitcoinTxIn, len(msgTx.TxIn))
		for j, in := range msgTx.TxIn {
			kernelTx.inputs[j] = &KernelBitcoinTxIn{
				txIn:     in,
				pkScript: nil, // This will be populated when we have access to previous outputs
				parentTx: kernelTx,
			}
		}
		txs[i] = kernelTx
	}
	return txs
}

func (b *KernelBlock) SetBlockHeight(height uint32) {
	b.blockHeight = height
}

// Ensure KernelBitcoinTransaction implements indexer.Transaction
var _ indexer.Transaction = (*KernelBitcoinTransaction)(nil)

type KernelBitcoinTransaction struct {
	tx      *wire.MsgTx
	inputs  []*KernelBitcoinTxIn
	outputs []*KernelBitcoinTxOut
}

func (t *KernelBitcoinTransaction) GetTxId() [32]byte {
	hash := t.tx.TxHash()
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

func (t *KernelBitcoinTransaction) GetTxIdSlice() []byte {
	hash := t.tx.TxHash()
	return hash[:]
}

func (t *KernelBitcoinTransaction) GetTxIns() []indexer.TxIn {
	ins := make([]indexer.TxIn, len(t.inputs))
	for i, in := range t.inputs {
		ins[i] = in
	}
	return ins
}

func (t *KernelBitcoinTransaction) GetTxOuts() []indexer.TxOut {
	outs := make([]indexer.TxOut, len(t.outputs))
	for i, out := range t.outputs {
		outs[i] = out
	}
	return outs
}

// Ensure KernelBitcoinTxIn implements indexer.TxIn
var _ indexer.TxIn = (*KernelBitcoinTxIn)(nil)

type KernelBitcoinTxIn struct {
	txIn     *wire.TxIn
	pkScript []byte
	parentTx *KernelBitcoinTransaction
}

func (in *KernelBitcoinTxIn) GetTxId() [32]byte {
	var result [32]byte
	copy(result[:], in.txIn.PreviousOutPoint.Hash[:])
	return result
}

func (in *KernelBitcoinTxIn) GetTxIdSlice() []byte {
	return in.txIn.PreviousOutPoint.Hash[:]
}

func (in *KernelBitcoinTxIn) GetVout() uint32 {
	return in.txIn.PreviousOutPoint.Index
}

func (in *KernelBitcoinTxIn) GetPrevoutPkScript() []byte {
	return in.pkScript
}

func (in *KernelBitcoinTxIn) SetPrevoutPkScript(script []byte) {
	in.pkScript = script
}

func (in *KernelBitcoinTxIn) GetWitness() [][]byte {
	return in.txIn.Witness
}

func (in *KernelBitcoinTxIn) GetScriptSig() []byte {
	return in.txIn.SignatureScript
}

// Ensure KernelBitcoinTxOut implements indexer.TxOut
var _ indexer.TxOut = (*KernelBitcoinTxOut)(nil)

type KernelBitcoinTxOut struct {
	txOut    *wire.TxOut
	outpoint *wire.OutPoint
	parentTx *KernelBitcoinTransaction
}

func (out *KernelBitcoinTxOut) GetTxId() [32]byte {
	var result [32]byte
	copy(result[:], out.outpoint.Hash[:])
	return result
}

func (out *KernelBitcoinTxOut) GetTxIdSlice() []byte {
	return out.outpoint.Hash[:]
}

func (out *KernelBitcoinTxOut) GetVout() uint32 {
	return out.outpoint.Index
}

func (out *KernelBitcoinTxOut) GetValue() uint64 {
	return uint64(out.txOut.Value)
}

func (out *KernelBitcoinTxOut) GetPkScript() []byte {
	return out.txOut.PkScript
}
