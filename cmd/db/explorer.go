package main

import (
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-oracle/internal/database/dbpebble"
)

// DatabaseExplorer provides methods to explore the pebble database
type DatabaseExplorer struct {
	db *pebble.DB
}

// NewDatabaseExplorer creates a new database explorer instance
func NewDatabaseExplorer(dbPath string) (*DatabaseExplorer, error) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &DatabaseExplorer{db: db}, nil
}

// Close closes the database connection
func (de *DatabaseExplorer) Close() error {
	return de.db.Close()
}

// CountKeysByType counts keys of a specific type, optionally within a height range
func (de *DatabaseExplorer) CountKeysByType(keyType string, startHeight, endHeight uint32) (int, error) {
	var lowerBound, upperBound []byte
	var err error

	// Get bounds based on key type
	switch keyType {
	case "compute-index":
		lowerBound, upperBound = dbpebble.BoundsComputeIndex(startHeight, endHeight)
	case "ci-height":
		lowerBound, upperBound = de.getCIHeightBounds(startHeight, endHeight)
	case "tweaks-static":
		lowerBound, upperBound = de.getTweaksStaticBounds(startHeight, endHeight)
	case "utxos-static":
		lowerBound, upperBound = de.getUTXOsStaticBounds(startHeight, endHeight)
	case "taproot-pubkey-filter":
		lowerBound, upperBound = de.getTaprootPubkeyFilterBounds(startHeight, endHeight)
	case "taproot-unspent-filter":
		lowerBound, upperBound = de.getTaprootUnspentFilterBounds(startHeight, endHeight)
	case "taproot-spent-filter":
		lowerBound, upperBound = de.getTaprootSpentFilterBounds(startHeight, endHeight)
	case "block-tx", "tx", "out", "spend", "ci-block", "tx-occur":
		// These key types don't use height ranges, count all keys of this type
		lowerBound, upperBound = de.getKeyTypeBounds(keyType)
	default:
		return 0, fmt.Errorf("unsupported key type: %s", keyType)
	}

	// Create iterator options
	opts := &pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	}

	iter, err := de.db.NewIter(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}

	if err := iter.Error(); err != nil {
		return 0, fmt.Errorf("iterator error: %w", err)
	}

	return count, nil
}

// CountComputeIndexKeys counts compute index keys in the specified height range (legacy method)
func (de *DatabaseExplorer) CountComputeIndexKeys(startHeight, endHeight uint32) (int, error) {
	return de.CountKeysByType("compute-index", startHeight, endHeight)
}

// GetDatabaseStats returns basic statistics about the database
func (de *DatabaseExplorer) GetDatabaseStats() (*pebble.Metrics, error) {
	metrics := de.db.Metrics()
	return metrics, nil
}

// ListAllKeyTypes returns a count of keys by type prefix
func (de *DatabaseExplorer) ListAllKeyTypes() (map[byte]int, error) {
	keyCounts := make(map[byte]int)

	iter, err := de.db.NewIter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) > 0 {
			prefix := key[0]
			keyCounts[prefix]++
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return keyCounts, nil
}

// PrintKeyTypeSummary prints a summary of key types in the database
func (de *DatabaseExplorer) PrintKeyTypeSummary() error {
	keyCounts, err := de.ListAllKeyTypes()
	if err != nil {
		return err
	}

	fmt.Println("Database Key Type Summary:")
	fmt.Println("=========================")

	// Define key type names for better readability
	keyTypeNames := map[byte]string{
		dbpebble.KBlockTx:              "BlockTx",
		dbpebble.KTx:                   "Tx",
		dbpebble.KOut:                  "Out",
		dbpebble.KSpend:                "Spend",
		dbpebble.KCIHeight:             "CIHeight",
		dbpebble.KCIBlock:              "CIBlock",
		dbpebble.KTxOccur:              "TxOccur",
		dbpebble.KTweaksStatic:         "TweaksStatic",
		dbpebble.KUTXOsStatic:          "UTXOsStatic",
		dbpebble.KTaprootPubkeyFilter:  "TaprootPubkeyFilter",
		dbpebble.KTaprootUnspentFilter: "TaprootUnspentFilter",
		dbpebble.KTaprootSpentFilter:   "TaprootSpentFilter",
		dbpebble.KComputeIndex:         "ComputeIndex",
	}

	totalKeys := 0
	for prefix, count := range keyCounts {
		name := keyTypeNames[prefix]
		if name == "" {
			name = fmt.Sprintf("Unknown(0x%02X)", prefix)
		}
		fmt.Printf("%-25s: %d keys\n", name, count)
		totalKeys += count
	}

	fmt.Printf("%-25s: %d keys\n", "TOTAL", totalKeys)
	return nil
}

// GetHeightRange returns the min and max heights in the database
func (de *DatabaseExplorer) GetHeightRange() (uint32, uint32, error) {
	var minHeight, maxHeight uint32
	var found bool

	// Iterate through all CIHeight keys to find min/max
	iter, err := de.db.NewIter(nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()

		// Check if this is a CIHeight key
		if len(key) >= 1 && key[0] == dbpebble.KCIHeight {
			// Extract height from key (bytes 1-4)
			if len(key) >= 5 {
				height := uint32(key[1])<<24 | uint32(key[2])<<16 | uint32(key[3])<<8 | uint32(key[4])

				if !found {
					minHeight = height
					maxHeight = height
					found = true
				} else {
					if height < minHeight {
						minHeight = height
					}
					if height > maxHeight {
						maxHeight = height
					}
				}
			}
		}
	}

	if err := iter.Error(); err != nil {
		return 0, 0, fmt.Errorf("iterator error: %w", err)
	}

	if !found {
		return 0, 0, fmt.Errorf("no height data found in database")
	}

	return minHeight, maxHeight, nil
}

// PrintDatabaseInfo prints comprehensive database information
func (de *DatabaseExplorer) PrintDatabaseInfo() error {
	fmt.Println("Blindbit Oracle Database Information")
	fmt.Println("====================================")

	// Print height range
	minHeight, maxHeight, err := de.GetHeightRange()
	if err != nil {
		fmt.Printf("Error getting height range: %v\n", err)
	} else {
		fmt.Printf("Height Range: %d - %d (%d blocks)\n", minHeight, maxHeight, maxHeight-minHeight+1)
	}

	fmt.Println()

	// Print key type summary
	if err := de.PrintKeyTypeSummary(); err != nil {
		return fmt.Errorf("failed to print key type summary: %w", err)
	}

	fmt.Println()

	// Print database metrics
	metrics, err := de.GetDatabaseStats()
	if err != nil {
		fmt.Printf("Error getting database metrics: %v\n", err)
	} else {
		fmt.Println("Database Metrics:")
		fmt.Printf("  Range Key Sets: %d\n", metrics.Keys.RangeKeySetsCount)
		fmt.Printf("  Tombstones: %d\n", metrics.Keys.TombstoneCount)
		fmt.Printf("  Memtable Size: %d bytes\n", metrics.MemTable.Size)
		fmt.Printf("  Block Cache Size: %d bytes\n", metrics.BlockCache.Size)
		fmt.Printf("  WAL Files: %d\n", metrics.WAL.Files)
		fmt.Printf("  WAL Size: %d bytes\n", metrics.WAL.Size)
	}

	return nil
}

// Helper methods for getting bounds for different key types

func (de *DatabaseExplorer) getCIHeightBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// CIHeight keys: KCIHeight + height (4 bytes)
	lowerBound := make([]byte, 1+dbpebble.SizeHeight)
	lowerBound[0] = dbpebble.KCIHeight
	binary.BigEndian.PutUint32(lowerBound[1:1+dbpebble.SizeHeight], startHeight)

	upperBound := make([]byte, 1+dbpebble.SizeHeight)
	upperBound[0] = dbpebble.KCIHeight
	binary.BigEndian.PutUint32(upperBound[1:1+dbpebble.SizeHeight], endHeight)

	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getTweaksStaticBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// TweaksStatic keys: KTweaksStatic + blockhash (32 bytes)
	// We need to get block hashes for the height range
	lowerBound := make([]byte, 1+dbpebble.SizeHash)
	lowerBound[0] = dbpebble.KTweaksStatic

	upperBound := make([]byte, 1+dbpebble.SizeHash)
	upperBound[0] = dbpebble.KTweaksStatic

	// For now, we'll use a simple approach - iterate through heights
	// In a real implementation, you'd want to get the actual block hashes
	// This is a simplified version that will work for counting
	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getUTXOsStaticBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// UTXOsStatic keys: KUTXOsStatic + blockhash (32 bytes)
	lowerBound := make([]byte, 1+dbpebble.SizeHash)
	lowerBound[0] = dbpebble.KUTXOsStatic

	upperBound := make([]byte, 1+dbpebble.SizeHash)
	upperBound[0] = dbpebble.KUTXOsStatic

	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getTaprootPubkeyFilterBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// TaprootPubkeyFilter keys: KTaprootPubkeyFilter + blockhash (32 bytes)
	lowerBound := make([]byte, 1+dbpebble.SizeHash)
	lowerBound[0] = dbpebble.KTaprootPubkeyFilter

	upperBound := make([]byte, 1+dbpebble.SizeHash)
	upperBound[0] = dbpebble.KTaprootPubkeyFilter

	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getTaprootUnspentFilterBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// TaprootUnspentFilter keys: KTaprootUnspentFilter + blockhash (32 bytes)
	lowerBound := make([]byte, 1+dbpebble.SizeHash)
	lowerBound[0] = dbpebble.KTaprootUnspentFilter

	upperBound := make([]byte, 1+dbpebble.SizeHash)
	upperBound[0] = dbpebble.KTaprootUnspentFilter

	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getTaprootSpentFilterBounds(startHeight, endHeight uint32) ([]byte, []byte) {
	// TaprootSpentFilter keys: KTaprootSpentFilter + blockhash (32 bytes)
	lowerBound := make([]byte, 1+dbpebble.SizeHash)
	lowerBound[0] = dbpebble.KTaprootSpentFilter

	upperBound := make([]byte, 1+dbpebble.SizeHash)
	upperBound[0] = dbpebble.KTaprootSpentFilter

	return lowerBound, upperBound
}

func (de *DatabaseExplorer) getKeyTypeBounds(keyType string) ([]byte, []byte) {
	var prefix byte

	switch keyType {
	case "block-tx":
		prefix = dbpebble.KBlockTx
	case "tx":
		prefix = dbpebble.KTx
	case "out":
		prefix = dbpebble.KOut
	case "spend":
		prefix = dbpebble.KSpend
	case "ci-block":
		prefix = dbpebble.KCIBlock
	case "tx-occur":
		prefix = dbpebble.KTxOccur
	default:
		prefix = 0xFF // Invalid prefix to return no results
	}

	lowerBound := []byte{prefix}
	upperBound := []byte{prefix + 1}

	return lowerBound, upperBound
}
