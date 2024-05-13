# Notes

This file is to keep track of changes made over time and to have reference points for the implementation. Very old
information has been removed. Specification details about the communication protocol between indexing server and light
clients can be found [here](https://github.com/setavenger/BIP0352-light-client-specification.git).

## Tweak Computation Performance

Results from Benchmarking. Running v2 is a clear win in terms of speed for all types of blocks (many txs and few txs).
Spinning up a go routine for every tweak seems very efficient. But can it be improved? Can push the performance even a
bit more?
Next I want to try to assign a number of tweaks to a goroutine before it spins up,
so that we don't have the overhead of a goroutine spinning up all the time.

We variations between different benchmarking calls,
as seen by v1 where the only one thread is used, and we still see some discrepancies.
Overall the pattern becomes clear. v4 reduces the overhead of goroutine spawning significantly but does not outperform
v2 in any real way.
v2 seems to be quite optimised at this point. I'm not quite sure what one could try to boost performance except of
course just utilizing more cores.
Using more cores clearly improves the performance (almost linearly in some cases).
Parallel processing could be used for extracting [spent UTXOs](./src/core/extractutxos.go) (L:31) as well.
This is not a priority at the moment as the processing time seems to be low.

It should be noted that these are benchmarking results when solely running the tweak computation function.
During initial syncing/indexing there are also a lot of parallel processes for the rpc calls.

The functions can be found [here](./src/core/tweak.go).

### 12 Goroutines

```text
goos: darwin
goarch: amd64
pkg: SilentPaymentAppBackend/src/core
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkTweakV4Block833000-16                66          18221455 ns/op
BenchmarkTweakV3Block833000-16                51          23240305 ns/op
BenchmarkTweakV2Block833000-16                66          18484002 ns/op
BenchmarkTweakV1Block833000-16                 8         158669041 ns/op
BenchmarkTweakV4Block833010-16                42          28893857 ns/op
BenchmarkTweakV3Block833010-16                30          37702025 ns/op
BenchmarkTweakV2Block833010-16                42          28723057 ns/op
BenchmarkTweakV1Block833010-16                 5         212243446 ns/op
BenchmarkTweakV4Block833013-16                44          28600250 ns/op
BenchmarkTweakV3Block833013-16                36          34166821 ns/op
BenchmarkTweakV2Block833013-16                42          28579243 ns/op
BenchmarkTweakV1Block833013-16                 6         190190890 ns/op
BenchmarkTweakV4Block834469-16                86          13207238 ns/op
BenchmarkTweakV3Block834469-16                91          12260387 ns/op
BenchmarkTweakV2Block834469-16                82          13145223 ns/op
BenchmarkTweakV1Block834469-16                15          75665007 ns/op
PASS
ok      SilentPaymentAppBackend/src/core        25.640s
```

### 6 Goroutines

```text
Allowed number of parallel processes (`common.MaxParallelTweakComputations`) was 6.

goos: darwin
goarch: amd64
pkg: SilentPaymentAppBackend/src/core
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkTweakV4Block833000-16                34          37608796 ns/op
BenchmarkTweakV3Block833000-16                31          37644976 ns/op
BenchmarkTweakV2Block833000-16                43          35005047 ns/op
BenchmarkTweakV1Block833000-16                 8         132897864 ns/op
BenchmarkTweakV4Block833010-16                24          58622605 ns/op
BenchmarkTweakV3Block833010-16                19          62589645 ns/op
BenchmarkTweakV2Block833010-16                21          52516952 ns/op
BenchmarkTweakV1Block833010-16                 5         204381619 ns/op
BenchmarkTweakV4Block833013-16                21          54992341 ns/op
BenchmarkTweakV3Block833013-16                18          57974175 ns/op
BenchmarkTweakV2Block833013-16                28          49971872 ns/op
BenchmarkTweakV1Block833013-16                 6         184793615 ns/op
BenchmarkTweakV4Block834469-16                66          21655617 ns/op
BenchmarkTweakV3Block834469-16                67          16031086 ns/op
BenchmarkTweakV2Block834469-16                50          20486003 ns/op
BenchmarkTweakV1Block834469-16                15          68968977 ns/op
PASS
ok      SilentPaymentAppBackend/src/core        27.134s
```
