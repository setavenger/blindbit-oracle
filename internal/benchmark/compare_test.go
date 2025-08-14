package benchmark

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/networking"
)

// TestCompareV1V2Results compares results from v1 HTTP and v2 gRPC endpoints
func TestCompareV1V2Results(t *testing.T) {
	// Configuration
	httpURL := "http://127.0.0.1:8000"
	grpcHost := "127.0.0.1:50051"
	testHeight := uint64(100) // Adjust to a height you know has data

	// Setup logging
	logging.SetLogLevel(zerolog.InfoLevel)

	t.Logf("Comparing v1 and v2 results for block height %d", testHeight)

	// Fetch data from v1 (HTTP)
	v1Data, err := fetchBlockDataV1(testHeight, &networking.ClientBlindBit{BaseURL: httpURL})
	if err != nil {
		t.Fatalf("Failed to fetch v1 data: %v", err)
	}

	// Fetch data from v2 (gRPC)
	v2Data, err := fetchBlockDataV2(testHeight, grpcHost)
	if err != nil {
		t.Fatalf("Failed to fetch v2 data: %v", err)
	}

	// Compare results
	t.Run("CompareTweaks", func(t *testing.T) {
		compareTweaks(t, v1Data.Tweaks, v2Data.Tweaks)
	})

	t.Run("CompareNewUTXOsFilter", func(t *testing.T) {
		compareFilter(t, "NewUTXOs", v1Data.FilterNew, v2Data.FilterNew)
	})

	t.Run("CompareSpentOutpointsFilter", func(t *testing.T) {
		compareFilter(t, "SpentOutpoints", v1Data.FilterSpent, v2Data.FilterSpent)
	})

	t.Log("All comparisons passed - v1 and v2 endpoints return identical data")
}

// compareTweaks compares tweak arrays from v1 and v2
func compareTweaks(t *testing.T, v1Tweaks, v2Tweaks [][33]byte) {
	if len(v1Tweaks) != len(v2Tweaks) {
		t.Errorf("Tweak count mismatch: v1=%d, v2=%d", len(v1Tweaks), len(v2Tweaks))
		return
	}

	t.Logf("Comparing %d tweaks", len(v1Tweaks))

	for i, v1Tweak := range v1Tweaks {
		if i >= len(v2Tweaks) {
			t.Errorf("v2 tweaks array too short at index %d", i)
			continue
		}

		v2Tweak := v2Tweaks[i]
		if v1Tweak != v2Tweak {
			t.Errorf("Tweak mismatch at index %d: v1=%x, v2=%x",
				i, v1Tweak, v2Tweak)
		}
	}
}

// compareFilter compares filter data from v1 and v2
func compareFilter(t *testing.T, filterName string, v1Filter, v2Filter *networking.Filter) {
	if v1Filter == nil && v2Filter == nil {
		t.Logf("%s filter: both nil (no data)", filterName)
		return
	}

	if v1Filter == nil {
		t.Errorf("%s filter: v1 is nil but v2 is not", filterName)
		return
	}

	if v2Filter == nil {
		t.Errorf("%s filter: v2 is nil but v1 is not", filterName)
		return
	}

	t.Logf("%s filter: comparing data", filterName)

	// Compare filter data
	if len(v1Filter.Data) != len(v2Filter.Data) {
		t.Errorf("%s filter data length mismatch: v1=%d, v2=%d",
			filterName, len(v1Filter.Data), len(v2Filter.Data))
		return
	}

	for i, v1Byte := range v1Filter.Data {
		if i >= len(v2Filter.Data) {
			t.Errorf("%s filter: v2 data too short at index %d", filterName, i)
			continue
		}

		v2Byte := v2Filter.Data[i]
		if v1Byte != v2Byte {
			t.Errorf("%s filter data mismatch at index %d: v1=%x, v2=%x",
				filterName, i, v1Byte, v2Byte)
		}
	}

	// Compare block hash
	if v1Filter.BlockHash != v2Filter.BlockHash {
		t.Errorf("%s filter block hash mismatch: v1=%x, v2=%x",
			filterName, v1Filter.BlockHash, v2Filter.BlockHash)
	}

	// Compare block height
	if v1Filter.BlockHeight != v2Filter.BlockHeight {
		t.Errorf("%s filter block height mismatch: v1=%d, v2=%d",
			filterName, v1Filter.BlockHeight, v2Filter.BlockHeight)
	}
}
