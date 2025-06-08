package dblevel

import "github.com/setavenger/blindbit-lib/logging"

func CloseDBs() {
	err := HeadersDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing headers db")
	}
	err = HeadersInvDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing headers inv db")
	}
	err = NewUTXOsFiltersDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing new utxos filters db")
	}
	err = TweaksDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweaks db")
	}
	err = TweakIndexDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweak index db")
	}
	err = TweakIndexDustDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing tweak index dust db")
	}
	err = UTXOsDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing utxos db")
	}
	err = SpentOutpointsIndexDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing spent outpoints index db")
	}
	err = SpentOutpointsFilterDB.Close()
	if err != nil {
		logging.L.Err(err).Msg("error closing spent outpoints filter db")
	}

	logging.L.Info().Msg("DBs closed")
}
