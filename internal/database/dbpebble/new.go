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
	opts.Cache = pebble.NewCache(512 << 23) // 512 MiB cache; tune as you like
	opts.BytesPerSync = 1 << 20             // smoother background flushes (1 MiB)  (SST sync pacing)

	// For initial sync you can also turn off the WAL for speed (see §2):
	// opts.DisableWAL = true // crash = reindex, but very fast

	opts.MaxConcurrentCompactions = func() int { return 10 }

	// opts.EventListener = &pebble.EventListener{
	// 	WriteStallBegin: func(info pebble.WriteStallBeginInfo) {
	// 		logging.L.Debug().Any("info", info).Msg("write_stall_end")
	// 	},
	// 	WriteStallEnd: func() {
	// 		logging.L.Debug().Any("info", nil).Msg("write_stall_end")
	// 	},
	// 	// CompactionBegin: func(info pebble.CompactionInfo) {
	// 	// 	logging.L.Debug().Any("info", info).Msg("compact_begin")
	// 	// },
	// 	// CompactionEnd: func(info pebble.CompactionInfo) {
	// 	// 	logging.L.Debug().Any("info", info).Msg("compact_end")
	// 	// },
	// }

	opts.BytesPerSync = 1 << 22

	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		return nil, err
	}

	// Periodic metrics log (every 5–10s)
	// go func() {
	// 	t := time.NewTicker(10 * time.Second)
	// 	defer t.Stop()
	// 	for range t.C {
	// 		m := db.Metrics()
	// 		// The pretty string shows per-level tables/sizes and the compaction debt/stalls.
	// 		// It’s the easiest way to see L0 pressure & stalls.
	// 		logging.L.Warn().Any("metrics", m).Msg("")
	// 	}
	// }()
	return db, err
}
