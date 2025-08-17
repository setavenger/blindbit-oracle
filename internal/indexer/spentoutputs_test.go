package indexer

import (
	"encoding/hex"
	"fmt"
	"testing"
)

// input data

var spentBytes, _ = hex.DecodeString("030001fcedf305000000002200204ae81572f06e1b88fd5ced7a1a000945432e83e1551e6f721ee9c00b8cc3326001d772cb1d000000002200204ae81572f06e1b88fd5ced7a1a000945432e83e1551e6f721ee9c00b8cc33260")

func TestDeserialise(t *testing.T) {
	spentTxs, err := ParseSpentTxOuts(spentBytes)
	if err != nil {
		t.Fatal(err)
	}

	for i := range len(spentTxs) {
		v := spentTxs[i]
		fmt.Printf("Tx: %d\n", i)
		for j := range v {
			fmt.Printf("%03d %x - %12d\n", j, v[j].PkScript, v[j].Value)
		}
	}
}
