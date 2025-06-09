package bitcoinkernel

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
	"github.com/setavenger/go-bitcoinkernel/pkg/bitcoinkernel"
)

// Ensure KernelBlock implements indexer.BlockData
var _ indexer.BlockData = (*KernelBlock)(nil)

type KernelBlock struct {
	btcutil.Block
	blockHeight  uint32
	blockUndo    *bitcoinkernel.BlockUndo
	transactions []indexer.Transaction // for caching once computed already
	numWorkers   int                   // number of concurrent workers
}

func NewKernelBlock(blockData []byte, height uint32, blockUndo *bitcoinkernel.BlockUndo) *KernelBlock {
	block, err := btcutil.NewBlockFromBytes(blockData)
	if err != nil {
		logging.L.Err(err).Msg("Failed to create block from bytes")
		return nil
	}
	return &KernelBlock{
		Block:       *block,
		blockHeight: height,
		blockUndo:   blockUndo,
		numWorkers:  8, // default number of workers
	}
}

// SetNumWorkers sets the number of concurrent workers for processing transactions
func (b *KernelBlock) SetNumWorkers(num int) {
	if num < 1 {
		num = 1
	}
	b.numWorkers = num
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
	if b.transactions != nil {
		// todo: should we always return a copy?
		return b.transactions
	}

	// skip coinbase tx (-1)
	txs := make([]indexer.Transaction, len(b.Transactions())-1)
	txsBlock := b.Transactions()
	// Skip coinbase transaction ([1:])
	// Create a channel to receive results
	type result struct {
		index int
		tx    indexer.Transaction
	}
	resultChan := make(chan result, len(txsBlock)-1)

	// Create a channel to limit concurrent goroutines
	semaphore := make(chan struct{}, b.numWorkers)

	// Process transactions in parallel
	for i, tx := range txsBlock[1:] {
		semaphore <- struct{}{} // Acquire semaphore
		go func(i int, tx *btcutil.Tx) {
			defer func() { <-semaphore }() // Release semaphore

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
				// Get prevout data from block undo
				txUndo, err := b.blockUndo.GetPrevoutByIndex(uint64(i), uint64(j))
				if err != nil {
					logging.L.Panic().
						Err(err).
						Hex("txid", kernelTx.GetTxIdSlice()).
						Msgf("Failed to get prevout by index (tx: %d, input: %d)", i, j)
					return
				}

				scriptPubkeyPrevout, err := txUndo.GetScriptPubkey()
				if err != nil {
					logging.L.
						Err(err).
						Hex("txid", kernelTx.GetTxIdSlice()).
						Msgf("Failed to get script pubkey (tx: %d, input: %d)", i, j)
					txUndo.Close()
					continue
				}

				scriptPubkeyPrevoutBytes, err := scriptPubkeyPrevout.GetData()
				if err != nil {
					logging.L.Err(err).Msgf("Failed to get script pubkey data (tx: %d, input: %d)", i, j)
					scriptPubkeyPrevout.Close()
					txUndo.Close()
					continue
				}

				kernelTx.inputs[j] = &KernelBitcoinTxIn{
					txIn:     in,
					pkScript: scriptPubkeyPrevoutBytes,
					parentTx: kernelTx,
				}

				scriptPubkeyPrevout.Close()
				txUndo.Close()
			}

			resultChan <- result{index: i, tx: kernelTx}
		}(i, tx)
	}

	// Collect results
	for i := 0; i < len(txsBlock)-1; i++ {
		result := <-resultChan
		txs[result.index] = result.tx
	}
	// todo: should we always return a copy?
	b.transactions = txs

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
	utils.ReverseBytes(result[:])
	return result
}

func (t *KernelBitcoinTransaction) GetTxIdSlice() []byte {
	hash := t.tx.TxHash()
	utils.ReverseBytes(hash[:])
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
	utils.ReverseBytes(result[:])
	return result
}

func (in *KernelBitcoinTxIn) GetTxIdSlice() []byte {
	hash := in.txIn.PreviousOutPoint.Hash
	utils.ReverseBytes(hash[:])
	return hash[:]
}

func (in *KernelBitcoinTxIn) GetVout() uint32 {
	return in.txIn.PreviousOutPoint.Index
}

func (in *KernelBitcoinTxIn) GetPrevoutPkScript() []byte {
	return in.pkScript
}

func (in *KernelBitcoinTxIn) GetPkScript() []byte {
	return in.pkScript
}

func (in *KernelBitcoinTxIn) GetWitness() [][]byte {
	return in.txIn.Witness
}

func (in *KernelBitcoinTxIn) GetScriptSig() []byte {
	return in.txIn.SignatureScript
}

func (in *KernelBitcoinTxIn) Valid() bool {
	// coinbase outpoint index is 0xffffffff
	return in.txIn.PreviousOutPoint.Index != 0xffffffff
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
