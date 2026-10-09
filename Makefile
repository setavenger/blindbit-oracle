# Benchmark targets
.PHONY: benchmark
benchmark: build-benchmark
	@echo "Running benchmark..."
	./bin/benchmark $(ARGS)

.PHONY: benchmark-v1
benchmark-v1: build-benchmark
	@echo "Running v1 HTTP benchmark only..."
	./bin/benchmark -v1 -v2=false $(ARGS)

.PHONY: benchmark-v2
benchmark-v2: build-benchmark
	@echo "Running v2 gRPC benchmark only..."
	./bin/benchmark -v1=false -v2 $(ARGS)

.PHONY: compare
compare: build-benchmark
	@echo "Comparing v1 and v2 data..."
	./bin/benchmark -compare $(ARGS)

.PHONY: build-benchmark
build-benchmark:
	@echo "Building benchmark tool..."
	@mkdir -p bin
	go build -o bin/benchmark ./cmd/benchmark

# Example usage:
# make benchmark ARGS="-startheight=100 -endheight=200"
# make benchmark-v1 ARGS="-startheight=100 -endheight=200"
# make benchmark-v2 ARGS="-startheight=100 -endheight=200"
# make compare ARGS="-startheight=100 -endheight=200"
