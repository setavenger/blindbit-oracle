package core

import (
	"fmt"
	"log"
	"testing"

	"github.com/setavenger/blindbit-oracle/src/common/types"
	"github.com/setavenger/blindbit-oracle/src/testhelpers"
)

func TestBlockAnalysis(t *testing.T) {
	var block types.Block
	err := testhelpers.LoadBlockFromFile("/Users/setorblagogee/dev/sp-test-dir/block-716120.json", &block)
	if err != nil {
		log.Fatalln(err)
	}

	tweaks, err := ComputeTweaksForBlock(&block)
	if err != nil {
		log.Fatalln(err)
	}

	for _, tweak := range tweaks {
		fmt.Printf("%x - %s\n", tweak.TweakData, tweak.Txid)
	}

	for _, tx := range block.Txs {
		for _, tweak := range tweaks {
			if tx.Txid == tweak.Txid {
				fmt.Printf("%x\n", tweak.TweakData)
			}
		}
	}

}
