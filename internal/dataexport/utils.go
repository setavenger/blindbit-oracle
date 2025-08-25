package dataexport

import (
	"fmt"
	"os"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

func ExportAll() {
	// todo manage memory better, bloats completely during export
	logging.L.Info().Msg("Exporting data")
	timestamp := time.Now()

	logging.L.Info().Msg("Exporting UTXOs")
	err := ExportUTXOs(fmt.Sprintf("%s/data-export/utxos-%d.csv", config.BaseDirectory, timestamp.Unix()))
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error exporting utxos")
	}
	logging.L.Info().Msg("Finished UTXOs")

	logging.L.Info().Msg("Exporting Filters")
	err = ExportFilters(fmt.Sprintf("%s/data-export/filters-%d.csv", config.BaseDirectory, timestamp.Unix()))
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error exporting filters")
	}
	logging.L.Info().Msg("Finished Filters")

	logging.L.Info().Msg("Exporting Tweaks")
	err = ExportTweaks(fmt.Sprintf("%s/data-export/tweaks-%d.csv", config.BaseDirectory, timestamp.Unix()))
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error exporting tweaks")
	}
	logging.L.Info().Msg("Finished Tweaks")

	logging.L.Info().Msg("Exporting Tweak Indices")
	err = ExportTweakIndices(fmt.Sprintf("%s/data-export/tweak-indices-%d.csv", config.BaseDirectory, timestamp.Unix()))
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error exporting tweak indices")
	}
	logging.L.Info().Msg("Finished Tweak Indices")

	logging.L.Info().Msg("Exporting HeadersInv")
	err = ExportHeadersInv(fmt.Sprintf("%s/data-export/headers-inv-%d.csv", config.BaseDirectory, timestamp.Unix()))
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error exporting headers inv")
	}
	logging.L.Info().Msg("Finished HeadersInv")

	logging.L.Info().Msg("Export Done")
	os.Exit(0)
}
