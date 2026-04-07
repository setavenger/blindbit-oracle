// Package dbpebble is a fast key-value implementation for the database.DB interface
//
// Fast initial syncs
package dbpebble

import (
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

func OpenDB() (*pebble.DB, error) {
	dbPath := filepath.Join(config.BaseDirectory, "pebbledb", "db")
	// during DB open
	opts := (&pebble.Options{}).EnsureDefaults()
	opts.Cache = pebble.NewCache(512 << 23) // 4 GiB
	opts.BytesPerSync = 1 << 22             // smoother background flushes (4 MiB)  (SST sync pacing)

	// For initial sync you can also turn off the WAL for speed (see §2):
	// opts.DisableWAL = true // crash = reindex, but very fast

	opts.MaxConcurrentCompactions = func() int { return 10 }

	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		return nil, err
	}

	return db, err
}
