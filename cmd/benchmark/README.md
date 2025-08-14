# Oracle Benchmark Tool

This tool benchmarks the performance difference between v1 (HTTP) and v2 (gRPC streaming) APIs for fetching block data.

## Building

```bash
make build-benchmark
```

## Usage

### Run both benchmarks
```bash
./bin/benchmark -startheight=100 -endheight=200
```

### Run only v1 HTTP benchmark
```bash
./bin/benchmark -v1 -v2=false -startheight=100 -endheight=200
```

### Run only v2 gRPC benchmark
```bash
./bin/benchmark -v1=false -v2 -startheight=100 -endheight=200
```

### Using Makefile
```bash
# Run both
make benchmark ARGS="-startheight=100 -endheight=200"

# Run only v1
make benchmark-v1 ARGS="-startheight=100 -endheight=200"

# Run only v2
make benchmark-v2 ARGS="-startheight=100 -endheight=200"
```

## Command Line Flags

- `-startheight`: Start block height (default: 1)
- `-endheight`: End block height (default: 10)
- `-http`: HTTP API base URL (default: "http://127.0.0.1:8000")
- `-grpc`: gRPC server host:port (default: "127.0.0.1:50051")
- `-v1`: Run v1 HTTP benchmark (default: true)
- `-v2`: Run v2 gRPC benchmark (default: true)

## What it measures

The benchmark fetches block data (tweaks, filters) for each block height in the range and measures:

- Total time to fetch all blocks
- Blocks processed per second
- Individual block fetch times

## Expected Results

- **v1 (HTTP)**: Makes individual HTTP requests for each block, good for small ranges
- **v2 (gRPC)**: Uses streaming to fetch all blocks in one connection, better for large ranges

The gRPC streaming approach should show better performance for larger block ranges due to reduced connection overhead and better batching.
