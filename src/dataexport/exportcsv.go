package dataexport

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"encoding/csv"
	"encoding/hex"
	"log"
	"os"
	"strconv"
)

func writeToCSV(path string, records [][]string) error {
	// Create a new file
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			common.ErrorLogger.Println(err)
		}
	}(file) // Ensure the file is closed at the end

	// Create a new CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush() // Flush writes any buffered data to the underlying io.Writer

	// Write all CSV records
	err = writer.WriteAll(records) // calls Flush internally
	if err != nil {
		common.ErrorLogger.Println("error writing record to csv:", err)
		return err
	}
	return err
}

// UTXOS

func ExportUTXOs(path string) error {
	allEntries, err := dblevel.FetchAllUTXOs()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	records, err := convertUTXOsToRecords(allEntries)
	if err != nil {
		common.ErrorLogger.Println(err)
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
	allEntries, err := dblevel.FetchAllFilters()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	records, err := convertFiltersToRecords(allEntries)
	if err != nil {
		common.ErrorLogger.Println(err)
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
		common.ErrorLogger.Println(err)
		return err
	}
	records, err := convertTweaksToRecords(allEntries)
	if err != nil {
		common.ErrorLogger.Println(err)
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
			hex.EncodeToString(pair.Data[:]),
		})
	}
	return records, nil
}

// TweakIndex

func ExportTweakIndices(path string) error {
	allEntries, err := dblevel.FetchAllTweakIndices()
	if err != nil {
		common.ErrorLogger.Println(err)
		return err
	}
	records, err := convertTweakIndicesToRecords(allEntries)
	if err != nil {
		common.ErrorLogger.Println(err)
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
		common.ErrorLogger.Println(err)
		return err
	}
	records, err := convertHeadersInvToRecords(allEntries)
	if err != nil {
		common.ErrorLogger.Println(err)
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
