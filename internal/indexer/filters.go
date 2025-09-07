package indexer

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/gcs"
	"github.com/btcsuite/btcutil/gcs/builder"
	"github.com/setavenger/blindbit-lib/logging"
)

// BuildTaprootOnlyFilter creates the taproot only filter
func BuildTaprootPubkeyFilter(blockhash []byte, taprootOutputs [][]byte) ([]byte, error) {
	// blockhash is already reversed
	c := chainhash.Hash{}
	err := c.SetBytes(blockhash)

	if err != nil {
		logging.L.Fatal().
			Err(err).Hex("blockhash", blockhash).
			Msg("failed to set block hash")
		return nil, err
	}
	key := builder.DeriveKey(&c)

	filter, err := gcs.BuildGCSFilter(builder.DefaultP, builder.DefaultM, key, taprootOutputs)
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", blockhash).Msg("failed to build GCS filter")
		return nil, err
	}

	nBytes, err := filter.NBytes()
	if err != nil {
		logging.L.Fatal().Err(err).Hex("blockhash", blockhash).Msg("failed to get NBytes")
		return nil, err
	}

	return nBytes, nil
}
