package main

import (
	"flag"

	"github.com/rs/zerolog"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/benchmark"
)

func main() {
	var (
		startHeight = flag.Uint64("startheight", 1, "Start block height")
		endHeight   = flag.Uint64("endheight", 10, "End block height")
		httpURL     = flag.String("http", "http://127.0.0.1:8000", "HTTP API base URL")
		grpcHost    = flag.String("grpc", "127.0.0.1:50051", "gRPC server host:port")
		runV1       = flag.Bool("v1", true, "Run v1 HTTP benchmark")
		runV2       = flag.Bool("v2", true, "Run v2 gRPC benchmark")
		compare     = flag.Bool("compare", false, "Compare v1 and v2 data instead of benchmarking")
	)
	flag.Parse()

	// Setup logging
	logging.SetLogLevel(zerolog.InfoLevel)

	if *compare {
		logging.L.Info().
			Uint64("start_height", *startHeight).
			Uint64("end_height", *endHeight).
			Msg("Starting data comparison")

		benchmark.CompareV1V2Results(*startHeight, *endHeight, *httpURL, *grpcHost)
		return
	}

	logging.L.Info().
		Uint64("start_height", *startHeight).
		Uint64("end_height", *endHeight).
		Msg("Starting benchmark")

	if *runV1 {
		logging.L.Info().Msg("=== Running V1 HTTP Benchmark ===")
		benchmark.BenchmarkV1(*startHeight, *endHeight, *httpURL)
	}

	if *runV2 {
		logging.L.Info().Msg("=== Running V2 gRPC Streaming Benchmark ===")
		benchmark.BenchmarkV2(*startHeight, *endHeight, *grpcHost)
	}

	logging.L.Info().Msg("Benchmark completed")
}
