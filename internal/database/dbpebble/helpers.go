package dbpebble

import (
	"encoding/binary"
)

func be32(u uint32, b []byte) { binary.BigEndian.PutUint32(b, u) }
func be64(u uint64, b []byte) { binary.BigEndian.PutUint64(b, u) }
func le64(u uint64, b []byte) { binary.LittleEndian.PutUint64(b, u) }
