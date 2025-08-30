package database

import "encoding/binary"

const OutputBinLength = 32 + 4 + 8 + 32

func (o *Output) BinarySerialisation() (out []byte) {
	// todo: should we return fixed size?
	out = make([]byte, OutputBinLength)
	copy(out[:32], o.Txid)
	binary.LittleEndian.PutUint32(out[32:36], o.Vout)
	binary.LittleEndian.PutUint64(out[36:44], o.Amount)
	copy(out[44:76], o.Pubkey)
	return out
}

func (o *Output) BinaryDeSerialisation(in []byte) {
	_ = in[OutputBinLength-1] //bounds check

	copy(o.Txid, in[:32])
	o.Vout = binary.LittleEndian.Uint32(in[32:36])
	o.Amount = binary.NativeEndian.Uint64(in[36:44])
	copy(o.Pubkey, in[44:OutputBinLength])
}
