package dataexport

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func writeToCSV(path string, records [][]string) error {
	// Create a new file
	os.MkdirAll(fmt.Sprintf("%s/export", config.BaseDirectory), 0750)
	file, err := os.Create(path)
	if err != nil {
		logging.L.Fatal().Err(err).Msg("failed creating file")
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			logging.L.Err(err).Msg("error closing file")
		}
	}(file) // Ensure the file is closed at the end

	// Create a new CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush() // Flush writes any buffered data to the underlying io.Writer

	// Write all CSV records
	err = writer.WriteAll(records) // calls Flush internally
	if err != nil {
		logging.L.Err(err).Msg("error writing record to csv")
		return err
	}
	return err
}

// UTXOS

func ExportUTXOs(path string) error {
	allEntries, err := dblevel.FetchAllUTXOs()
	if err != nil {
		logging.L.Err(err).Msg("error fetching all utxos")
		return err
	}
	records, err := convertUTXOsToRecords(allEntries)
	if err != nil {
		logging.L.Err(err).Msg("error converting utxos to records")
		return err
	}
	return writeToCSV(path, records)
}

func convertUTXOsToRecords(utxos []types.UTXO) ([][]string, error) {
	var records [][]string

	records = append(records, []string{
		"blockHash",
		"txid",
		"vout",
		"scriptPubKey",
		"value",
	})
	for _, pair := range utxos {
		records = append(records, []string{
			pair.BlockHash,
			pair.Txid,
			strconv.FormatUint(uint64(pair.Vout), 10),
			pair.ScriptPubKey,
			strconv.FormatUint(pair.Value, 10),
		})
	}
	return records, nil
}

// Filters

func ExportFilters(path string) error {
	allEntries, err := dblevel.FetchAllNewUTXOsFilters()
	if err != nil {
		logging.L.Err(err).Msg("error fetching all new utxos filters")
		return err
	}
	records, err := convertFiltersToRecords(allEntries)
	if err != nil {
		logging.L.Err(err).Msg("error converting filters to records")
		return err
	}
	return writeToCSV(path, records)
}

func convertFiltersToRecords(data []types.Filter) ([][]string, error) {
	var records [][]string

	records = append(records, []string{
		"blockHash",
		"filterType",
		"data",
	})
	for _, pair := range data {
		records = append(records, []string{
			pair.BlockHash,
			strconv.FormatUint(uint64(pair.FilterType), 10),
			hex.EncodeToString(pair.Data),
		})
	}
	return records, nil
}

// Tweaks

func ExportTweaks(path string) error {
	allEntries, err := dblevel.FetchAllTweaks()
	if err != nil {
		logging.L.Err(err).Msg("error fetching all tweaks")
		return err
	}
	records, err := convertTweaksToRecords(allEntries)
	if err != nil {
		logging.L.Err(err).Msg("error converting tweaks to records")
		return err
	}
	return writeToCSV(path, records)
}

func convertTweaksToRecords(data []types.Tweak) ([][]string, error) {
	var records [][]string

	records = append(records, []string{
		"blockHash",
		"txid",
		"data",
	})
	for _, pair := range data {
		records = append(records, []string{
			pair.BlockHash,
			pair.Txid,
			hex.EncodeToString(pair.TweakData[:]),
		})
	}
	return records, nil
}

// TweakIndex

func ExportTweakIndices(path string) error {
	allEntries, err := dblevel.FetchAllTweakIndices()
	if err != nil {
		logging.L.Err(err).Msg("error fetching all tweak indices")
		return err
	}
	records, err := convertTweakIndicesToRecords(allEntries)
	if err != nil {
		logging.L.Err(err).Msg("error converting tweak indices to records")
		return err
	}
	return writeToCSV(path, records)
}

func convertTweakIndicesToRecords(data []types.TweakIndex) ([][]string, error) {
	var records [][]string

	records = append(records, []string{
		"blockHash",
		"data",
	})
	for _, pair := range data {
		// todo can this be made more efficiently?
		totalLength := len(pair.Data) * 33
		flattened := make([]byte, 0, totalLength)

		for _, byteArray := range pair.Data {
			flattened = append(flattened, byteArray[:]...)
		}

		records = append(records, []string{
			pair.BlockHash,
			hex.EncodeToString(flattened),
		})
	}
	return records, nil
}

// HeadersInv

func ExportHeadersInv(path string) error {
	allEntries, err := dblevel.FetchAllHeadersInv()
	if err != nil {
		logging.L.Err(err).Msg("error fetching all headers inv")
		return err
	}
	records, err := convertHeadersInvToRecords(allEntries)
	if err != nil {
		logging.L.Err(err).Msg("error converting headers inv to records")
		return err
	}
	return writeToCSV(path, records)
}

func convertHeadersInvToRecords(data []types.BlockHeaderInv) ([][]string, error) {
	var records [][]string

	records = append(records, []string{
		"blockHeight",
		"blockHash",
	})
	for _, pair := range data {
		records = append(records, []string{
			strconv.FormatUint(uint64(pair.Height), 10),
			pair.Hash,
		})
	}
	return records, nil
}
