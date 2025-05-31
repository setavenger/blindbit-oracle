package core

import (
	"testing"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/testhelpers"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func TestBlockAnalysis(t *testing.T) {
	var block types.Block
	err := testhelpers.LoadBlockFromFile("/Users/setorblagogee/dev/sp-test-dir/block-716120.json", &block)
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error loading block from file")
		t.FailNow()
	}

	tweaks, err := ComputeTweaksForBlock(&block)
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error computing tweaks for block")
		t.FailNow()
	}

	for _, tweak := range tweaks {
		logging.L.Info().Bytes("tweak", tweak.TweakData).Str("txid", tweak.Txid).Msg("tweak")
	}

	for _, tx := range block.Txs {
		for _, tweak := range tweaks {
			if tx.Txid == tweak.Txid {
				logging.L.Info().Hex("tweak", tweak.TweakData).Msg("tweak")
			}
		}
	}
}
