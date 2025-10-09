package main

import (
	"encoding/binary"
	"encoding/hex"
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

// LookupKey looks up a specific key in the database and returns its value
func (de *DatabaseExplorer) LookupKey(keyType, keyData string, hexFormat bool) ([]byte, error) {
	// Convert key data to bytes
	var keyBytes []byte
	var err error

	if hexFormat {
		keyBytes, err = hex.DecodeString(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex key: %w", err)
		}
	} else {
		keyBytes = []byte(keyData)
	}

	// Get the prefix byte for the key type
	prefix, err := de.getKeyTypePrefix(keyType)
	if err != nil {
		return nil, err
	}

	var fullKey []byte

	// Check if the key already has the prefix byte
	if len(keyBytes) > 0 && keyBytes[0] == prefix {
		// Key already includes the prefix, use as-is
		fullKey = keyBytes
	} else {
		// Construct the full key with prefix
		fullKey = make([]byte, 1+len(keyBytes))
		fullKey[0] = prefix
		copy(fullKey[1:], keyBytes)
	}

	// Perform the lookup
	value, closer, err := de.db.Get(fullKey)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, nil // Key not found
		}
		return nil, fmt.Errorf("database lookup failed: %w", err)
	}
	defer closer.Close()

	// Return a copy of the value
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// IterateKeyRange iterates through a range of keys of a specific type
func (de *DatabaseExplorer) IterateKeyRange(keyType, startKey string, limit int, showValues bool) error {
	// Get the prefix byte for the key type
	prefix, err := de.getKeyTypePrefix(keyType)
	if err != nil {
		return err
	}

	// Create iterator options
	var opts *pebble.IterOptions

	if startKey != "" {
		// Convert start key to bytes
		startKeyBytes, err := hex.DecodeString(startKey)
		if err != nil {
			return fmt.Errorf("failed to decode hex start key: %w", err)
		}

		var fullStartKey []byte

		// Check if the key already has the prefix byte
		if len(startKeyBytes) > 0 && startKeyBytes[0] == prefix {
			// Key already includes the prefix, use as-is
			fullStartKey = startKeyBytes
		} else {
			// Create full start key with prefix
			fullStartKey = make([]byte, 1+len(startKeyBytes))
			fullStartKey[0] = prefix
			copy(fullStartKey[1:], startKeyBytes)
		}

		opts = &pebble.IterOptions{
			LowerBound: fullStartKey,
		}
	} else {
		// Start from the beginning of this key type
		opts = &pebble.IterOptions{
			LowerBound: []byte{prefix},
			UpperBound: []byte{prefix + 1},
		}
	}

	iter, err := de.db.NewIter(opts)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid() && count < limit; iter.Next() {
		key := iter.Key()

		// Verify this is the correct key type (in case we're not using bounds)
		if len(key) > 0 && key[0] == prefix {
			count++

			if showValues {
				value := iter.Value()
				fmt.Printf("%d: Key: %x, Value: %x\n", count, key, value)
			} else {
				fmt.Printf("%d: Key: %x\n", count, key)
			}
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	fmt.Printf("Iterated through %d %s keys\n", count, keyType)
	return nil
}

// ScanKeyPrefix scans keys that start with a specific prefix
func (de *DatabaseExplorer) ScanKeyPrefix(keyType, prefix string, limit int, showValues bool) error {
	// Get the prefix byte for the key type
	keyTypePrefix, err := de.getKeyTypePrefix(keyType)
	if err != nil {
		return err
	}

	// Convert prefix to bytes
	prefixBytes, err := hex.DecodeString(prefix)
	if err != nil {
		return fmt.Errorf("failed to decode hex prefix: %w", err)
	}

	// Create full prefix with key type prefix
	fullPrefix := make([]byte, 1+len(prefixBytes))
	fullPrefix[0] = keyTypePrefix
	copy(fullPrefix[1:], prefixBytes)

	// Create iterator options for prefix scanning
	opts := &pebble.IterOptions{
		LowerBound: fullPrefix,
	}

	iter, err := de.db.NewIter(opts)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid() && count < limit; iter.Next() {
		key := iter.Key()

		// Check if key starts with our full prefix
		if len(key) >= len(fullPrefix) {
			matches := true
			for i, b := range fullPrefix {
				if key[i] != b {
					matches = false
					break
				}
			}

			if matches {
				count++

				if showValues {
					value := iter.Value()
					fmt.Printf("%d: Key: %x, Value: %x\n", count, key, value)
				} else {
					fmt.Printf("%d: Key: %x\n", count, key)
				}
			} else {
				// No more keys with this prefix, break
				break
			}
		} else {
			// Key too short, break
			break
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	fmt.Printf("Found %d %s keys with prefix %s\n", count, keyType, prefix)
	return nil
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
		dbpebble.KBlockTx:      "BlockTx",
		dbpebble.KTx:           "Tx",
		dbpebble.KOut:          "Out",
		dbpebble.KSpend:        "Spend",
		dbpebble.KCIHeight:     "CIHeight",
		dbpebble.KCIBlock:      "CIBlock",
		dbpebble.KTxOccur:      "TxOccur",
		dbpebble.KComputeIndex: "ComputeIndex",
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

// getKeyTypePrefix returns the prefix byte for a given key type
func (de *DatabaseExplorer) getKeyTypePrefix(keyType string) (byte, error) {
	switch keyType {
	case "block-tx":
		return dbpebble.KBlockTx, nil
	case "tx":
		return dbpebble.KTx, nil
	case "out":
		return dbpebble.KOut, nil
	case "spend":
		return dbpebble.KSpend, nil
	case "ci-height":
		return dbpebble.KCIHeight, nil
	case "ci-block":
		return dbpebble.KCIBlock, nil
	case "tx-occur":
		return dbpebble.KTxOccur, nil
	case "compute-index":
		return dbpebble.KComputeIndex, nil
	default:
		return 0, fmt.Errorf("unsupported key type: %s", keyType)
	}
}
