package core

import (
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/testhelpers"
	"log"
	"testing"
)

/*

Results from Benchmarking. Running v2 is a clear win in terms of speed for all types of blocks (many txs and few txs)


goos: darwin
goarch: amd64
pkg: SilentPaymentAppBackend/src/core
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkTweakV2Block833000-16                40          32978726 ns/op
BenchmarkTweakV1Block833000-16                 8         132168333 ns/op
BenchmarkTweakV2Block833010-16                21          55258107 ns/op
BenchmarkTweakV1Block833010-16                 5         219691875 ns/op
BenchmarkTweakV2Block833013-16                27          51755626 ns/op
BenchmarkTweakV1Block833013-16                 6         191223854 ns/op
BenchmarkTweakV2Block834469-16                56          21750344 ns/op
BenchmarkTweakV1Block834469-16                16          70707631 ns/op
PASS
ok      SilentPaymentAppBackend/src/core        13.452s
*/

var (
	block833000, block833010, block833013, block834469 types.Block
)

func init() {
	err := testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833000.json", &block833000)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833010.json", &block833010)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833013.json", &block833013)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_834469.json", &block834469)
	if err != nil {
		log.Fatalln(err)
	}
}

func BenchmarkTweakV2Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
