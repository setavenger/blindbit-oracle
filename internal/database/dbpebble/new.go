package dbpebble

import (
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

func OpenDB() (*pebble.DB, error) {
	dbPath := filepath.Join(config.BaseDirectory, "pebbledb", "db")
	return pebble.Open(dbPath, &pebble.Options{})
}
