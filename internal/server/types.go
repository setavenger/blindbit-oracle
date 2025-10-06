package server

import (
	"encoding/hex"
	"encoding/json"
)

type BlockIdentifier struct {
	BlockHash   []byte `json:"block_hash"`
	BlockHeight uint32 `json:"block_height"`
}

func (b BlockIdentifier) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		BlockHash   string `json:"block_hash"`
		BlockHeight uint32 `json:"block_height"`
	}{
		BlockHash:   hex.EncodeToString(b.BlockHash),
		BlockHeight: b.BlockHeight,
	})
}

type FullBlockResponse struct {
	BlockIdentifier BlockIdentifier     `json:"block_identifier"`
	Index           []FullTxItem        `json:"index"`
	SpentOutpoints  SpentOutpointsIndex `json:"spent_outpoints"`
}

// TweakIndexResponse is tweak index response
// It does not contain the txid, for txids use ComputeIndex
type TweakIndexResponse struct {
	BlockIdentifier BlockIdentifier `json:"block_identifier"`
	Index           TweakSlice      `json:"index"`
}

// ComputeIndexResponse ComputeIndexItem array for block
type ComputeIndexResponse struct {
	BlockIdentifier BlockIdentifier    `json:"block_identifier"`
	Index           []ComputeIndexItem `json:"index"`
}

type TweakSlice [][33]byte

func (t TweakSlice) MarshalJSON() ([]byte, error) {
	out := make([]string, len(t))
	for i := range t {
		out[i] = hex.EncodeToString(t[i][:])
	}

	return json.Marshal(out)
}

// OutputsShort is a slice of 8 bytes each
// Will be marshalled to one contiguous hex string
type OutputsShort [][8]byte // 8 bytes each

func (o OutputsShort) MarshalJSON() ([]byte, error) {
	out := make([]string, len(o))
	for i := range len(o) {
		out[i] = hex.EncodeToString(o[i][:])
	}
	return json.Marshal(out)
}

// ComputeIndexItem contains compact information
// to probe if a txid could have an interesting new output
type ComputeIndexItem struct {
	TxId         [32]byte     `json:"txid"`
	Tweak        [33]byte     `json:"tweak"`
	OutputsShort OutputsShort `json:"outputs"`
}

func (c ComputeIndexItem) MarshalJSON() ([]byte, error) {
	outputsShortBytes, err := c.OutputsShort.MarshalJSON()
	if err != nil {
		return nil, err
	}

	// todo: can this be done without unmarshalling?
	// basically marshalling twice?
	var outputsShortStr []string
	json.Unmarshal(outputsShortBytes, &outputsShortStr)

	return json.Marshal(struct {
		TxId         string   `json:"txid"`
		Tweak        string   `json:"tweak"`
		OutputsShort []string `json:"outputs"`
	}{
		TxId:         hex.EncodeToString(c.TxId[:]),
		Tweak:        hex.EncodeToString(c.Tweak[:]),
		OutputsShort: outputsShortStr,
	})
}

type SpentIndexResponse struct {
	BlockIdentifier BlockIdentifier `json:"block_identifier"`
	Index           SpentIndex      `json:"index"`
}

// SpentIndex is a slice of 8 bytes each
type SpentIndex [][8]byte // 8 bytes each

func (s SpentIndex) MarshalJSON() ([]byte, error) {
	out := make([]string, len(s))
	for i := range s {
		out[i] = hex.EncodeToString(s[i][:])
	}
	return json.Marshal(out)
}

// SpentOutpointsIndex is a slice of 36 bytes each (32-byte txid + 4-byte vout)
type SpentOutpointsIndex [][36]byte // 36 bytes each

func (s SpentOutpointsIndex) MarshalJSON() ([]byte, error) {
	out := make([]string, len(s))
	for i := range s {
		out[i] = hex.EncodeToString(s[i][:])
	}
	return json.Marshal(out)
}

// FullTxItem is a struct that contains the information for a full
// Will be sent for Full Block Batch lots of data,
// should be avoided if possible
type FullTxItem struct {
	TxId  [32]byte        `json:"txid"`
	Tweak [33]byte        `json:"tweak"`
	UTXOs []UTXOItemLight `json:"utxos"` // should probably be optional
}

func (f FullTxItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TxId  string          `json:"txid"`
		Tweak string          `json:"tweak"`
		UTXOs []UTXOItemLight `json:"utxos"`
	}{
		TxId:  hex.EncodeToString(f.TxId[:]),
		Tweak: hex.EncodeToString(f.Tweak[:]),
		UTXOs: f.UTXOs,
	})
}

type UTXOItemLight struct {
	Vout   uint32   `json:"vout"`
	Amount uint64   `json:"amount"`
	Pubkey [32]byte `json:"pubkey"`
}

func (u UTXOItemLight) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Vout   uint32 `json:"vout"`
		Amount uint64 `json:"amount"`
		Pubkey string `json:"pubkey"`
	}{
		Vout:   u.Vout,
		Amount: u.Amount,
		Pubkey: hex.EncodeToString(u.Pubkey[:]),
	})
}

type UTXOItem struct {
	TxId   [32]byte `json:"txid,omitempty"`
	Vout   uint32   `json:"vout"`
	Amount uint64   `json:"amount"`
	Pubkey [32]byte `json:"pubkey"`
}

func (u UTXOItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TxId   string `json:"txid,omitempty"`
		Vout   uint32 `json:"vout"`
		Amount uint64 `json:"amount"`
		Pubkey string `json:"pubkey"`
	}{
		TxId:   hex.EncodeToString(u.TxId[:]),
		Vout:   u.Vout,
		Amount: u.Amount,
		Pubkey: hex.EncodeToString(u.Pubkey[:]),
	})
}
