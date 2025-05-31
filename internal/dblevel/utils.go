package dblevel

import (
	"fmt"
	"os"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/dataexport"
	"github.com/setavenger/blindbit-oracle/internal/db/dblevel"
)

func CloseDBs() {
	err := dblevel.HeadersDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing headers db")
	}
	err = dblevel.HeadersInvDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing headers inv db")
	}
	err = dblevel.NewUTXOsFiltersDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing new utxos filters db")
	}
	err = dblevel.TweaksDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweaks db")
	}
	err = dblevel.TweakIndexDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweak index db")
	}
	err = dblevel.TweakIndexDustDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweak index dust db")
	}
	err = dblevel.UTXOsDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing utxos db")
	}
	err = dblevel.SpentOutpointsIndexDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing spent outpoints index db")
	}
	err = dblevel.SpentOutpointsFilterDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing spent outpoints filter db")
	}

	logging.L.Info().Msg("DBs closed")
}

func ExportAll() {
	// todo manage memory better, bloats completely during export
	logging.L.Info().Msg("Exporting data")
	timestamp := time.Now()

	err := dataexport.ExportUTXOs(fmt.Sprintf("./data-export/utxos-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	logging.L.Info().Msg("Finished UTXOs")

	err = dataexport.ExportFilters(fmt.Sprintf("./data-export/filters-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	logging.L.Info().Msg("Finished Filters")

	err = dataexport.ExportTweaks(fmt.Sprintf("./data-export/tweaks-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	logging.L.Info().Msg("Finished Tweaks")

	err = dataexport.ExportTweakIndices(fmt.Sprintf("./data-export/tweak-indices-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	logging.L.Info().Msg("Finished Tweak Indices")

	err = dataexport.ExportHeadersInv(fmt.Sprintf("./data-export/headers-inv-%d.csv", timestamp.Unix()))
	if err != nil {
		panic(err)
	}
	logging.L.Info().Msg("Finished HeadersInv")

	logging.L.Info().Msg("All exported")
	os.Exit(0)
}
